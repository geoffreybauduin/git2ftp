package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/alecthomas/kingpin"
	"github.com/jlaffaye/ftp"
)

type GitFileInfo struct {
	Action string
	File   string
}

type App struct {
	GitDirectory    *string
	RemoteDirectory *string
	FromSHA         *string
	ToSHA           *string
	SyncDirectory   *string
	FTPUrl          *string
	FTPUser         *string
	FTPPassword     *string
}

func (a *App) normalize() error {
	if *app.ToSHA == "HEAD" {
		return fmt.Errorf("cannot use HEAD as value for --to-sha")
	}
	if *app.SyncDirectory != "" && (*app.SyncDirectory)[len(*app.SyncDirectory)-1] != '/' {
		*app.SyncDirectory += "/"
	}
	if *app.RemoteDirectory != "" && (*app.RemoteDirectory)[len(*app.RemoteDirectory)-1] != '/' {
		*app.RemoteDirectory += "/"
	}
	if (a.FTPUser == nil && a.FTPPassword != nil) || (a.FTPUser != nil && a.FTPPassword == nil) {
		return fmt.Errorf("ftp-user must be specified with ftp-password")
	}
	return nil
}

var (
	appPtr = kingpin.New("git2ftp", "Transfer your git commits to a distant FTP server")
	app    = &App{
		GitDirectory:    appPtr.Flag("git-directory", "Root directory of the git repository on your local machine").Required().String(),
		RemoteDirectory: appPtr.Flag("remote-directory", "Remote directory where you want to upload your files").Required().String(),
		FromSHA:         appPtr.Flag("from-sha", "Manually specify the git commit SHA to synchronize from").String(),
		ToSHA:           appPtr.Flag("to-sha", "Manually specific the git commit SHA to synchronize to. Don't use HEAD").Required().String(),
		SyncDirectory:   appPtr.Flag("sync-directory", "Directory to synchronize. Must be relative to git-directory. Defaults to '.'").Default("").String(),
		FTPUrl:          appPtr.Flag("ftp-url", "URL of the FTP, of the form: ftp.example.org:21").Required().String(),
		FTPUser:         appPtr.Flag("ftp-user", "User to log on the FTP").String(),
		FTPPassword:     appPtr.Flag("ftp-password", "Password for the user to log on the FTP").String(),
	}
)

func main() {
	if _, err := appPtr.Parse(os.Args[1:]); err != nil {
		exit(1, err, "cannot parse args")
	} else if err := app.normalize(); err != nil {
		exit(1, err)
	}

	git2ftpFile := path.Join(*app.RemoteDirectory, ".git2ftp")

	ftpCli := &FTP{}
	/*cli, err := ftp.Dial(*app.FTPUrl, ftp.DialWithTimeout(5*time.Second))
	if err != nil {
		exit(1, err, "cannot dial to ftp")
	}
	defer func() {
		if err := cli.Quit(); err != nil {
			exit(1, err, "cannot logout from ftp")
		}
	}()
	if app.FTPUser != nil {
		err = cli.Login(*app.FTPUser, *app.FTPPassword)
		if err != nil {
			exit(1, err, "cannot login to ftp")
		}
	}
	ftpCli.ServerConn = cli
	*/
	if app.FromSHA == nil {
		r, err := ftpCli.Retr(git2ftpFile)
		if err != nil {
			exit(1, err, "cannot retrieve .git2ftp file")
		}
		content, err := ioutil.ReadAll(r)
		if err != nil {
			exit(1, err, "cannot read from .git2ftp file")
		}
		strContent := string(content)
		app.FromSHA = &strContent
	}

	cmd := exec.Command("git", "diff", "--name-status", *app.FromSHA, *app.ToSHA)
	cmd.Dir = *app.GitDirectory
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		exit(1, err, "cannot pipe stdin")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		exit(1, err, "cannot pipe stderr")
	}
	err = cmd.Start()
	if err != nil {
		exit(1, err, "cannot start git diff command")
	}
	changedFiles := make([]GitFileInfo, 0)
	for {
		var info GitFileInfo
		n, err := fmt.Fscanf(stdout, "%s	%s", &info.Action, &info.File)
		if err != nil && err != io.EOF {
			panic(err)
		} else if n == 0 || err == io.EOF {
			break
		}
		if !strings.HasPrefix(info.File, *app.SyncDirectory) {
			continue
		}
		changedFiles = append(changedFiles, info)
	}
	outputErr, errRead := ioutil.ReadAll(stderr)
	if errRead != nil {
		exit(1, errRead, "cannot read from stderr")
	}
	err = cmd.Wait()
	if err != nil {
		// we should have something on stderr
		exit(1, fmt.Errorf("%s: %s", err, string(outputErr)))
	}
	for _, file := range changedFiles {
		if err := file.Apply(ftpCli); err != nil {
			exit(1, err, fmt.Sprintf("cannot upload file %s to ftp", file.File))
		}
	}
	if err := ftpCli.Stor(git2ftpFile, bytes.NewBufferString(*app.ToSHA)); err != nil {
		exit(1, err, "could not store current sha in ftp remote directory")
	}
}

func exit(code int, err error, prefix ...string) {
	pre := strings.Join(prefix, ", ")
	if pre != "" {
		pre = pre + ": "
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", pre, err.Error())
	os.Exit(code)
}

func (file GitFileInfo) Apply(ftpCli *FTP) error {
	remoteFile := strings.Replace(file.File, *app.SyncDirectory, *app.RemoteDirectory, 1)
	log.Printf("file %s remote equivalent is %s", file.File, remoteFile)
	switch file.Action {
	case "D":
		// delete file from remote
		return ftpCli.Delete(remoteFile)
	case "M", "A":
		log.Printf("STOR %s", remoteFile)
		localFile, err := os.Open(path.Join(*app.GitDirectory, file.File))
		if err != nil {
			return err
		}
		defer localFile.Close()
		return ftpCli.Stor(remoteFile, localFile)
	}
	return fmt.Errorf("unknown action: %s", file.Action)
}

type FTP struct {
	*ftp.ServerConn
}

func (f *FTP) Stor(remoteFile string, r io.Reader) error {
	log.Printf("STOR %s", remoteFile)
	if f.ServerConn == nil {
		return nil
	}
	return f.ServerConn.Stor(remoteFile, r)
}

func (f *FTP) Retr(remoteFile string) (io.Reader, error) {
	log.Printf("RETR %s", remoteFile)
	if f.ServerConn == nil {
		return bytes.NewBufferString(""), nil
	}
	return f.ServerConn.Retr(remoteFile)
}

func (f *FTP) Delete(remoteFile string) error {
	log.Printf("DEL %s", remoteFile)
	if f.ServerConn == nil {
		return nil
	}
	return f.ServerConn.Delete(remoteFile)
}
