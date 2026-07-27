package env

type Requirement struct {
	Key          string `json:"key"`
	RelativePath string `json:"relativePath"`
	Label        string `json:"label"`
	ImportName   string `json:"importName"`
}

func DefaultRequirements() []Requirement {
	return []Requirement{
		{
			Key:          "python",
			RelativePath: "python.exe",
			Label:        "Python 解释器",
			ImportName:   "",
		},
		{
			Key:          "matplotlib",
			RelativePath: "Lib/site-packages/matplotlib",
			Label:        "Matplotlib",
			ImportName:   "matplotlib",
		},
		{
			Key:          "pyqt5",
			RelativePath: "Lib/site-packages/PyQt5",
			Label:        "PyQt5",
			ImportName:   "PyQt5",
		},
	}
}
