# git2ftp

Synchornize a GIT repository on a FTP, using git diff to compute the changes needed

```
usage: git2ftp --git-directory=GIT-DIRECTORY --remote-directory=REMOTE-DIRECTORY --to-sha=TO-SHA --ftp-url=FTP-URL [<flags>]

Transfer your git commits to a distant FTP server

Flags:
  --help                         Show context-sensitive help (also try --help-long and --help-man).
  --git-directory=GIT-DIRECTORY  Root directory of the git repository on your local machine
  --remote-directory=REMOTE-DIRECTORY  
                                 Remote directory where you want to upload your files
  --from-sha=FROM-SHA            Manually specify the git commit SHA to synchronize from
  --to-sha=TO-SHA                Manually specific the git commit SHA to synchronize to. Don't use HEAD
  --sync-directory=""            Directory to synchronize. Must be relative to git-directory. Defaults to '.'
  --ftp-url=FTP-URL              URL of the FTP, of the form: ftp.example.org:21
  --ftp-user=FTP-USER            User to log on the FTP
  --ftp-password=FTP-PASSWORD    Password for the user to log on the FTP
```

## Examples

### With --from-sha

```
git2ftp --git-directory=. --remote-directory=/www/ --to-sha 89ac765c61753ea84a039fd45ff66e0134990241 --sync-directory www --ftp-url ftp.example.org:21 --ftp-user admin --ftp-password password --from-sha 949b3cb22e7620d7a2cc1381d47043bdfd018267
```

Given the current git repository:

```
$ ls -l
total 8
-rw-r--r--   1 geoffrey  staff   64 Dec 27 00:10 README.md
drwxr-xr-x  28 geoffrey  staff  896 Dec 27 00:49 scripts
drwxr-xr-x  28 geoffrey  staff  896 Dec 27 00:49 www
```

This command will upload the changes of the `www` directory to the remote directory `/www` between the git commit `949b3cb22e7620d7a2cc1381d47043bdfd018267` and `89ac765c61753ea84a039fd45ff66e0134990241`.

### Without --from-sha

```
git2ftp --git-directory=. --remote-directory=/www/ --to-sha 89ac765c61753ea84a039fd45ff66e0134990241 --sync-directory www --ftp-url ftp.example.org:21 --ftp-user admin --ftp-password password
```

Given the current git repository:

```
$ ls -l
total 8
-rw-r--r--   1 geoffrey  staff   64 Dec 27 00:10 README.md
drwxr-xr-x  28 geoffrey  staff  896 Dec 27 00:49 scripts
drwxr-xr-x  28 geoffrey  staff  896 Dec 27 00:49 www
```

This command will upload the changes of the `www` directory to the remote directory `/www` between the git commit supplied in the `/www/.git2ftp` and `89ac765c61753ea84a039fd45ff66e0134990241`.

## The .git2ftp file

This file contains the SHA of the last commit you have synchronized on the FTP remote directory.

## License

MIT License

Copyright (c) 2019 Geoffrey Bauduin

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
