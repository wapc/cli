package commands

type UpgradeCmd struct {
}

func (c *UpgradeCmd) Run(ctx *Context) error {
	wapcHome, err := ensureHomeDirectory()
	if err != nil {
		return err
	}

	return checkDependencies(wapcHome, true)
}
