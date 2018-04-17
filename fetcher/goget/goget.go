package goget

import (
	"fmt"
	"github.com/sirupsen/logrus"

	"github.com/gogap/builder/fetcher"
	"github.com/gogap/builder/utils"
	"github.com/gogap/config"
)

type GoGetFetcher struct {
}

func init() {
	fetcher.RegisterFetcher("goget", NewGoGetFetcher)
}

func NewGoGetFetcher(config.Configuration) (f fetcher.Fetcher, err error) {
	return &GoGetFetcher{}
}

func (p *GoGetFetcher) Fetch(url, revision string, update bool, repoConf config.Configuration) (err error) {

	args := repoConf.GetStringList("args")
	strGOPATH := utils.GoPath()

	if len(strGOPATH) == 0 {
		err = fmt.Errorf("GOPATH is empty")
		return
	}

	_, repoDir, exist := utils.FindPkgPathByGOPATH(strGOPATH, url)

	if !exist {

		err = utils.GoGet(url, args...)
		if err != nil {
			return
		}

		update = false

		logrus.WithField("fetcher", "goget").WithField("url", url).WithField("revision", revision).Infoln("Fetched")
	}

	if len(revision) > 0 {
		err = utils.GitCheckout(repoDir, revision)
		if err != nil {
			return
		}

		logrus.WithField("fetcher", "goget").WithField("url", url).WithField("revision", revision).Infoln("Checked out")
	}

	if update {

		var deteched bool
		deteched, err = utils.GitDetached(repoDir)
		if err != nil {
			return
		}

		if !deteched {
			err = utils.GitPull(repoDir)
			if err != nil {
				return
			}
			logrus.WithField("fetcher", "goget").WithField("url", url).WithField("revision", revision).Infoln("Updated")
		} else {
			logrus.WithField("fetcher", "goget").WithField("url", url).WithField("revision", revision).Warnln("Repo detetched, update skipped")
		}
	}

	return
}
