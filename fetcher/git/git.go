package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/gogap/builder/fetcher"
	"github.com/gogap/builder/utils"
	"github.com/gogap/config"
)

type GitFetcher struct {
}

func init() {
	fetcher.RegisterFetcher("git", NewGitFetcher)
}

func NewGitFetcher(config.Configuration) (f fetcher.Fetcher, err error) {
	return &GitFetcher{}, nil
}

func (p *GitFetcher) Fetch(url, revision string, update bool, repoConf config.Configuration) (err error) {

	repoName := utils.GitRepoName(url)
	if len(repoName) == 0 {
		err = fmt.Errorf("parse git url repository name failure")
		return
	}

	dir := repoConf.GetString("dir")
	if len(dir) == 0 {
		err = fmt.Errorf("unknown dir of repo: %s", url)
		return
	}

	strGOPATH := utils.GoPath()
	if len(strGOPATH) == 0 {
		err = fmt.Errorf("GOPATH is empty")
		return
	}

	absWorkDir := ""
	needClone := true
	pkgPath := filepath.Join(dir, repoName)
	gopath, absPkgDir, existDir := utils.FindPkgPathByGOPATH(strGOPATH, pkgPath)

	if !existDir {
		gopaths := strings.Split(strGOPATH, ":")
		gopath = gopaths[0]
		absWorkDir = filepath.Join(gopath, "src", dir)
		absPkgDir = filepath.Join(gopath, "src", pkgPath)

		err = os.MkdirAll(absWorkDir, 0755)
		if err != nil {
			return
		}
	} else {
		needClone = false
		absWorkDir = filepath.Join(gopath, "src", dir)
	}

	if needClone {
		err = utils.GitClone(absWorkDir, url, repoConf.GetStringList("args.clone")...)
		if err != nil {
			return
		}
		update = false
		logrus.WithField("fetcher", "git").WithField("url", url).WithField("revision", revision).Infoln("Fetched")
	}

	if len(revision) > 0 {
		err = utils.GitCheckout(absPkgDir, revision)
		if err != nil {
			return
		}
		logrus.WithField("fetcher", "git").WithField("url", url).WithField("revision", revision).Infoln("Checked out")
	}

	if update {
		var deteched bool
		deteched, err = utils.GitDetached(absPkgDir)
		if err != nil {
			return
		}

		if !deteched {
			err = utils.GitPull(absPkgDir, repoConf.GetStringList("args.pull")...)
			if err != nil {
				return
			}
			logrus.WithField("fetcher", "git").WithField("url", url).WithField("revision", revision).Infoln("Updated")
		} else {
			logrus.WithField("fetcher", "git").WithField("url", url).WithField("revision", revision).Warnln("Repo detetched, update skipped")
		}
	}

	return
}
