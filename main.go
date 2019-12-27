package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/textproto"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

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
	cli, err := ftp.Dial(*app.FTPUrl, ftp.DialWithTimeout(5*time.Second))
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

	if app.FromSHA == nil || *app.FromSHA == "" {
		r, err := ftpCli.Retr(git2ftpFile)
		if err != nil {
			if errProto, ok := err.(*textproto.Error); ok && errProto.Code == ftp.StatusFileUnavailable {
				exit(1, fmt.Errorf("file .git2ftp does not exist, you must specify --from-sha"))
			}
			exit(1, err, "cannot retrieve .git2ftp file")
		}
		content, err := ioutil.ReadAll(r)
		if err != nil {
			exit(1, err, "cannot read from .git2ftp file")
		}
		strContent := string(content)
		app.FromSHA = &strContent
	}

	log.Printf("Running git diff --name-status %s %s", *app.FromSHA, *app.ToSHA)
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
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		var info GitFileInfo
		txt := scanner.Text()
		info.Action = txt[0:1]
		info.File = strings.TrimSpace(txt[1:])
		if !strings.HasPrefix(info.File, *app.SyncDirectory) {
			continue
		}
		changedFiles = append(changedFiles, info)
	}
	if err := scanner.Err(); err != nil {
		exit(1, err)
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
		localFile, err := os.Open(path.Join(*app.GitDirectory, file.File))
		if err != nil {
			return err
		}
		defer localFile.Close()
		err = ftpCli.Stor(remoteFile, localFile)
		if err != nil {
			if errProto, ok := err.(*textproto.Error); ok && errProto.Code == ftp.StatusBadFileName {
				// directory does not exist
				dir, _ := path.Split(remoteFile)
				if errMkd := createDir(ftpCli, dir, false); errMkd != nil {
					return errMkd
				}
				err = ftpCli.Stor(remoteFile, localFile)
				if err == nil {
					return nil
				}
			}
			return err
		}
		return nil
	}
	return fmt.Errorf("unknown action: %s", file.Action)
}

type FTP struct {
	*ftp.ServerConn
}

func createDir(ftpCli *FTP, dir string, circuitBreaker bool) error {
	dir = strings.TrimRight(dir, "/")
	errMkd := ftpCli.MakeDir(dir)
	if errMkd != nil {
		if errMkdProto, ok := errMkd.(*textproto.Error); !circuitBreaker && ok && errMkdProto.Code == ftp.StatusFileUnavailable {
			split := strings.Split(dir, "/")
			if len(split) == 1 {
				return fmt.Errorf("cannot create dir")
			}
			if errCreate := createDir(ftpCli, strings.Join(split[:len(split)-1], "/"), false); errCreate != nil {
				return errCreate
			}
			return createDir(ftpCli, dir, true) // circuit breaker
		}
		return errMkd
	}
	return nil
}

func (f *FTP) Stor(remoteFile string, r io.Reader) error {
	logString := fmt.Sprintf("STOR %s", remoteFile)
	log.Println(logString)
	if f.ServerConn == nil {
		return nil
	}
	err := f.ServerConn.Stor(remoteFile, r)
	if err != nil {
		f.logError(logString, err)
		return err
	} else {
		f.logSuccess(logString)
	}
	return nil
}

func (f *FTP) logError(str string, err error) {
	if errP, ok := err.(*textproto.Error); ok {
		log.Printf("%s: %d\n", str, errP.Code)
	} else {
		log.Printf("%s: %s\n", str, err)
	}
}

func (f *FTP) logSuccess(str string) {
	log.Printf("%s: 200\n", str)
}

func (f *FTP) Retr(remoteFile string) (io.Reader, error) {
	logString := fmt.Sprintf("STOR %s", remoteFile)
	log.Println(logString)
	if f.ServerConn == nil {
		return bytes.NewBufferString(""), nil
	}
	r, err := f.ServerConn.Retr(remoteFile)
	if err != nil {
		f.logError(logString, err)
		return nil, err
	} else {
		f.logSuccess(logString)
	}
	return r, nil
}

func (f *FTP) Delete(remoteFile string) error {
	logString := fmt.Sprintf("DEL %s", remoteFile)
	log.Println(logString)
	if f.ServerConn == nil {
		return nil
	}
	err := f.ServerConn.Delete(remoteFile)
	if err != nil {
		f.logError(logString, err)
		return err
	} else {
		f.logSuccess(logString)
	}
	return nil
}

func (f *FTP) MakeDir(path string) error {
	logString := fmt.Sprintf("MKD %s", path)
	log.Println(logString)
	if f.ServerConn == nil {
		return nil
	}
	err := f.ServerConn.MakeDir(path)
	if err != nil {
		f.logError(logString, err)
		return err
	} else {
		f.logSuccess(logString)
	}
	return nil
}

func (f *FTP) List(path string) ([]*ftp.Entry, error) {
	logString := fmt.Sprintf("LIST %s", path)
	log.Println(logString)
	if f.ServerConn == nil {
		return []*ftp.Entry{}, nil
	}
	lst, err := f.ServerConn.List(path)
	if err != nil {
		f.logError(logString, err)
		return nil, err
	} else {
		f.logSuccess(logString)
	}
	return lst, nil
}
