package builder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"

	"github.com/gogap/builder/fetcher"
	"github.com/gogap/builder/utils"
	"github.com/gogap/config"
)

type PackageRevision struct {
	Package  string `json:"package"`
	Branch   string `json:"branch"`
	Revision string `json:"revision"`
}

type Project struct {
	Name     string
	conf     config.Configuration
	fetchers map[string]fetcher.Fetcher
	builder  *Builder
}

type Metadata struct {
	Name      string
	Packages  []string
	Confing   config.Configuration
	Revisions []PackageRevision
}

func NewProject(projName string, builder *Builder) (proj *Project, err error) {

	conf := builder.options.Config.GetConfig(projName)

	if conf.IsEmpty() {
		err = fmt.Errorf("could not inital project of %s config because of config is empty", projName)
		return
	}

	fetchers := make(map[string]fetcher.Fetcher)
	fetchersConf := conf.GetConfig("fetchers")

	if fetchersConf == nil {
		err = fmt.Errorf("could not inital project of %s config because of fetchers config is not set", projName)
		return
	}

	for _, fetcherName := range fetcher.Fetchers() {
		var f fetcher.Fetcher
		f, err = fetcher.NewFetcher(
			fetcherName,
			fetchersConf.GetConfig(fetcherName),
		)

		if err != nil {
			return
		}

		fetchers[fetcherName] = f
	}

	proj = &Project{
		Name:     projName,
		conf:     conf,
		fetchers: fetchers,
		builder:  builder,
	}

	return
}

func (p *Project) Pull() (err error) {
	repos, err := p.getFetchRepos()
	if err != nil {
		return
	}

	for _, repo := range repos {
		err = repo.Pull()
		if err != nil {
			return
		}
	}

	return
}

func (p *Project) Build(data map[string]interface{}, run bool, runArgs []string) (err error) {
	pkgs := p.conf.GetStringList("packages")

	if len(pkgs) == 0 {
		return
	}

	tempDir := fmt.Sprintf("%s/%s", os.TempDir(), uuid.New())

	err = os.MkdirAll(tempDir, 0755)
	if err != nil {
		return
	}

	mainFileName := fmt.Sprintf("main_%s.go", p.Name)
	importsFileName := fmt.Sprintf("main_%s_%s.go", p.Name, "imports")

	importsFilePath := filepath.Join(tempDir, importsFileName)
	mainFilePath := filepath.Join(tempDir, mainFileName)

	bufImports := bytes.NewBuffer(nil)

	bufImports.WriteString("package main\n")

	for _, pkg := range pkgs {
		bufImports.WriteString(fmt.Sprintf("import _ \"%s\"\n", pkg))
	}

	err = ioutil.WriteFile(importsFilePath, bufImports.Bytes(), 0644)
	if err != nil {
		err = fmt.Errorf("write %s failure to temp dir: %s", importsFilePath, err)
		return
	}

	defer os.Remove(importsFilePath)

	revisions := p.revisions(tempDir)

	bufMain := bytes.NewBuffer(nil)

	if data == nil {
		data = make(map[string]interface{})
	}

	data["Metadata"] = Metadata{
		Name:      p.Name,
		Packages:  pkgs,
		Confing:   p.conf,
		Revisions: revisions,
	}

	err = p.builder.options.Template.Execute(
		bufMain,
		data,
	)

	if err != nil {
		return
	}

	err = ioutil.WriteFile(mainFilePath, bufMain.Bytes(), 0644)
	if err != nil {
		err = fmt.Errorf("write %s failure to temp dir: %s", mainFilePath, err)
		return
	}

	// defer os.Remove(mainFilePath)

	// go get before build
	appendGetArgs := p.conf.GetStringList("build.args.go-get")
	gogetArgs := []string{"get", "-d"}
	gogetArgs = append(gogetArgs, appendGetArgs...)

	err = utils.ExecCommandSTDWD("go", tempDir, gogetArgs...)
	if err != nil {
		return
	}

	// go build
	appendBuildArgs := p.conf.GetStringList("build.args.go-build")
	buildArgs := []string{"build"}
	buildArgs = append(buildArgs, appendBuildArgs...)

	targetConf := p.conf.GetConfig("build.target")

	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	if targetConf.IsEmpty() || run {

		outputPath := filepath.Join(cwd, p.Name)
		if run {
			outputPath = filepath.Join(tempDir, p.Name)
		}

		buildArgs = append(buildArgs, "-o", outputPath, mainFilePath, importsFilePath)
		err = utils.ExecCommandSTD("go", nil, buildArgs...)
		if err != nil {
			return
		}

		if run {
			err = utils.ExecCommandSTD(outputPath, nil, runArgs...)
			return
		}
	} else {
		for _, targetOS := range targetConf.Keys() {
			targetArchs := targetConf.GetStringList(targetOS)
			for _, targetArch := range targetArchs {

				envs := []string{"GOOS=" + targetOS, "GOARCH=" + targetArch}
				targetBuildArgs := append(buildArgs, "-o", filepath.Join(cwd, fmt.Sprintf("%s-%s-%s", p.Name, targetOS, targetArch)), mainFilePath, importsFilePath)

				err = utils.ExecCommandSTD("go", envs, targetBuildArgs...)
				if err != nil {
					return
				}
			}
		}
	}

	return
}

func (p *Project) getFetchRepos() (repos []*Repo, err error) {
	reposConf := p.conf.GetConfig("repos")

	if reposConf == nil {
		return
	}

	var fetchRepos []*Repo

	for _, repoName := range reposConf.Keys() {
		repoConf := reposConf.GetConfig(repoName)
		if repoConf == nil {
			err = fmt.Errorf("repo's config is nil, project: %s, repo: %s", p.Name, repoName)
			return
		}

		url := repoConf.GetString("url")

		if len(url) == 0 {
			err = fmt.Errorf("repo's url is empty, project: %s, repo: %s", p.Name, repoName)
			return
		}

		f, exist := p.fetchers[repoConf.GetString("fetcher", "goget")]
		if !exist {
			err = fmt.Errorf("fetcher %s not exist, project: %s, repo: %s", p.fetchers[repoConf.GetString("fetcher", "goget")], p.Name, repoName)
			return
		}

		revision := repoConf.GetString("revision")

		r := &Repo{
			repoConf:   repoConf,
			Url:        url,
			Fetcher:    f,
			Revision:   revision,
			NeedUpdate: p.builder.options.UpdateRepo,
		}

		fetchRepos = append(fetchRepos, r)
	}

	repos = fetchRepos

	return
}

func (p *Project) revisions(wkdir string) []PackageRevision {

	pkgs, _ := utils.GoDeps(wkdir)

	if len(pkgs) == 0 {
		return nil
	}

	var pkgsRevision []PackageRevision

	strGOPATH := utils.GoPath()

	gopaths := strings.Split(strGOPATH, ":")

	goroot := utils.GoRoot()

	if len(goroot) > 0 {
		gopaths = append(gopaths, goroot)
	}

	revExists := map[string]bool{}

	for _, pkg := range pkgs {

		_, pkgPath, exist := utils.FindPkgPathByGOPATH(strGOPATH, pkg)

		if !exist {
			logrus.WithField("package", pkg).WithField("project", p.Name).WithField("pkg_path", pkgPath).Debugln("Package not found")
			continue
		}

		pkgHash, err := utils.GetCommitSHA(pkgPath)
		if err != nil {
			logrus.WithField("package", pkg).WithField("project", p.Name).WithError(err).WithField("pkg_path", pkgPath).Debugln("Get commit sha failure")
			continue
		}

		branchName, err := utils.GetBranchOrTagName(pkgPath)
		if err != nil {
			logrus.WithField("package", pkg).WithField("project", p.Name).WithError(err).WithField("pkg_path", pkgPath).Debugln("Get branch or tag name failure")
		}

		if !revExists[pkg] {
			revExists[pkg] = true
			pkgsRevision = append(pkgsRevision, PackageRevision{Package: pkg, Revision: pkgHash, Branch: branchName})
		}
	}

	return pkgsRevision
}
