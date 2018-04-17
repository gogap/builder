package builder

import (
	"fmt"
	"text/template"

	"github.com/sirupsen/logrus"

	"github.com/gogap/config"
)

type Builder struct {
	options      *Options
	projects     map[string]*Project
	projectsKeys []string
}

type Option func(*Options)

type Options struct {
	Config     config.Configuration
	UpdateRepo bool
	Template   *template.Template
}

func ConfigFile(file string) Option {
	return func(o *Options) {
		o.Config = config.NewConfig(config.ConfigFile(file))
	}
}

func WithConfig(conf config.Configuration) Option {
	return func(o *Options) {
		o.Config = conf
	}
}

func ConfigString(configStr string) Option {
	return func(o *Options) {
		o.Config = config.NewConfig(config.ConfigString(configStr))
	}
}

func Template(tmpl *template.Template) Option {
	return func(o *Options) {
		o.Template = tmpl
	}
}

func UpdateRepo(update bool) Option {
	return func(o *Options) {
		o.UpdateRepo = update
	}
}

func NewBuilder(opts ...Option) (builder *Builder, err error) {
	builderOpts := &Options{}

	for _, o := range opts {
		o(builderOpts)
	}

	var projs = make(map[string]*Project)
	var projKeys []string

	bu := &Builder{options: builderOpts}

	for _, projName := range builderOpts.Config.Keys() {
		var proj *Project
		proj, err = NewProject(projName, bu)
		if err != nil {
			return
		}

		if _, exist := projs[projName]; exist {
			if exist {
				err = fmt.Errorf("project: %s already exist", projName)
				return
			}
		}

		projs[projName] = proj
		projKeys = append(projKeys, projName)
	}

	bu.projectsKeys = projKeys
	bu.projects = projs

	builder = bu

	return
}

func (p *Builder) ListProject() []string {
	var porj []string
	for _, c := range p.projectsKeys {
		porj = append(porj, c)
	}
	return porj
}

func (p *Builder) Build(data map[string]interface{}, porj ...string) (err error) {
	for _, c := range porj {
		logrus.WithField("project", c).Infoln("building")
		err = p.projects[c].Build(data, false, nil)
		if err != nil {
			return
		}
	}
	return
}

func (p *Builder) Run(data map[string]interface{}, porj string, args []string) (err error) {
	err = p.projects[porj].Build(data, true, args)
	if err != nil {
		return
	}
	return
}

func (p *Builder) Pull(porj ...string) (err error) {
	for _, c := range porj {
		err = p.projects[c].Pull()
		if err != nil {
			return
		}
	}
	return
}
