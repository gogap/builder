package builder

import (
	"github.com/gogap/builder/fetcher"
	"github.com/gogap/config"
)

type Repo struct {
	repoConf   config.Configuration
	Fetcher    fetcher.Fetcher
	Url        string
	Revision   string
	NeedUpdate bool
}

func (p *Repo) Pull() (err error) {
	err = p.Fetcher.Fetch(p.Url, p.Revision, p.NeedUpdate, p.repoConf)
	return
}
