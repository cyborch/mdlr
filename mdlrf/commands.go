package mdlrf

import (
	"fmt"
	"os"
)

type MdlrCtx struct {
	IsFileReady bool
	FilePath    string
	MdlrFile    *MdlrFile
}

func NewMdlrCtxForCmd() (*MdlrCtx, error) {
	c := &MdlrCtx{}
	var err error
	c.FilePath, err = getMdlrFilePathForCmd()
	return c, err
}

func (ctx *MdlrCtx) loadFile() error {
	if ctx.MdlrFile != nil || ctx.IsFileReady {
		return ErrMdlrFileAlreadyLoaded
	}
	if ctx.FilePath == "" {
		return ErrMdlrFileInvalidPath
	}
	ctx.MdlrFile = &MdlrFile{}
	err := ctx.MdlrFile.Load(ctx.FilePath)
	if err != nil {
		ctx.MdlrFile = nil
		return err
	}
	ctx.IsFileReady = true
	return nil
}

func (ctx *MdlrCtx) Init() error {
	if err := ctx.loadFile(); err == nil {
		return ErrMdlrFileAlreadyExists
	} else if err != ErrMdlrFileNotExist {
		return err
	}
	ctx.MdlrFile = NewMdlrFile()
	ctx.MdlrFile.Prepare(ctx.FilePath)
	ctx.IsFileReady = true
	return ctx.MdlrFile.Persist()
}

func (ctx *MdlrCtx) List() (string, error) {
	err := ctx.loadFile()
	if err != nil {
		return "", err
	}
	if len(ctx.MdlrFile.Modules) == 0 {
		return "There aren't any modules defined in the mdlr.yml file yet. Try running the add command with mdlr to add a module.", nil
	}
	items := make([]string, 0, len(ctx.MdlrFile.Modules))
	for _, m := range ctx.MdlrFile.Modules {
		items = append(items, fmt.Sprintf("%s:%s (%s) %s@%s(%s) [current=%s]", m.Name, m.Path, m.Type, m.URL, m.Branch, m.Commit, m.Status(true)))
	}
	out := fmt.Sprintf("Modules: %d", len(items))
	for n, val := range items {
		out += fmt.Sprintf("\n%d. %s", n+1, val)
	}
	return out, nil
}

func (ctx *MdlrCtx) Add(name string, mType string, path string, url string, branch string, commit string) error {
	err := ctx.loadFile()
	if err != nil {
		return err
	}
	if _, exist := ctx.MdlrFile.Modules[name]; exist {
		return ErrModuleNameAlreadyInUse
	}
	ctx.MdlrFile.Modules[name] = &Module{
		Type:   mType,
		Path:   path,
		URL:    url,
		Branch: branch,
		Commit: commit,
	}
	ctx.MdlrFile.Modules[name].Prepare(name, ctx.MdlrFile.ParentDirectory)
	err = ctx.MdlrFile.Modules[name].Validate()
	if err != nil {
		return err
	}
	return ctx.MdlrFile.Persist()
}

func (ctx *MdlrCtx) Remove(name string, dropFiles bool) error {
	err := ctx.loadFile()
	if err != nil {
		return err
	}
	if len(ctx.MdlrFile.Modules) == 0 {
		return ErrNoModules
	}
	if _, exist := ctx.MdlrFile.Modules[name]; !exist {
		return ErrModuleNameNotExist
	}
	dirPath := ctx.MdlrFile.Modules[name].AbsolutePath
	delete(ctx.MdlrFile.Modules, name)
	if dropFiles {
		err := os.RemoveAll(dirPath)
		if err != nil {
			return err
		}
	}
	return ctx.MdlrFile.Persist()
}

func (ctx *MdlrCtx) Import(specificName string, force bool) error {
	err := ctx.loadFile()
	if err != nil {
		return err
	}
	if len(ctx.MdlrFile.Modules) == 0 {
		return ErrNoModules
	}
	var runForRepo = func(name string) error {
		if _, exist := ctx.MdlrFile.Modules[name]; !exist {
			return ErrModuleNameNotExist
		}
		if force {
			dirPath := ctx.MdlrFile.Modules[name].AbsolutePath
			os.RemoveAll(dirPath)
		}
		return ctx.MdlrFile.Modules[name].Import(ctx.MdlrFile.Modules[name].Branch, ctx.MdlrFile.Modules[name].Commit, ctx.MdlrFile.Modules[name].Depth)
	}
	if specificName != "" {
		err := runForRepo(specificName)
		if err != nil {
			return err
		}
	} else {
		for _, m := range ctx.MdlrFile.Modules {
			if err := runForRepo(m.Name); err != nil {
				return err
			}
		}
	}
	return ctx.MdlrFile.Persist()
}

func (ctx *MdlrCtx) Update(specificName, branch, commit string, force bool) error {
	if commit == "" {
		commit = "HEAD"
	}
	var err error
	if force {
		err = ctx.Import(specificName, force)
		if err != nil {
			return err
		}
	} else {
		err = ctx.loadFile()
		if err != nil {
			return err
		}
		if len(ctx.MdlrFile.Modules) == 0 {
			return ErrNoModules
		}
	}
	var runForRepo = func(name string) error {
		if _, exist := ctx.MdlrFile.Modules[name]; !exist {
			return ErrModuleNameNotExist
		}
		b := branch
		if b == "" {
			b = ctx.MdlrFile.Modules[name].Branch
		}
		c, err := ctx.MdlrFile.Modules[name].Update(b, commit)
		if err != nil {
			return err
		}
		Log.Println("COMMIT:", c)
		ctx.MdlrFile.Modules[name].Branch = b
		if ctx.MdlrFile.Modules[name].Commit != "HEAD" {
			ctx.MdlrFile.Modules[name].Commit = c
		}
		return nil
	}
	if specificName != "" {
		err := runForRepo(specificName)
		if err != nil {
			return err
		}
	} else {
		for _, m := range ctx.MdlrFile.Modules {
			if err := runForRepo(m.Name); err != nil {
				return err
			}
		}
	}
	return ctx.MdlrFile.Persist()
}

func (ctx *MdlrCtx) Status(name string) (string, error) {
	err := ctx.loadFile()
	if err != nil {
		return "", err
	}
	if len(ctx.MdlrFile.Modules) == 0 {
		return "", ErrNoModules
	}
	if _, exist := ctx.MdlrFile.Modules[name]; !exist {
		return "", ErrModuleNameNotExist
	}
	return ctx.MdlrFile.Modules[name].Status(false), nil
}
