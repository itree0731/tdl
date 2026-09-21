package tui

type PickerMode uint8

const (
	PickDirectory PickerMode = iota
	PickFiles
	PickFilesAndDirectories
	PickOpenFile
	PickSaveFile
)

type PickerRequest struct {
	Purpose    string
	Mode       PickerMode
	InitialDir string
	Multi      bool
	Recursive  bool
	AllowedExt []string
	ShowHidden bool
}

type SelectedFile struct {
	Path    string
	Size    int64
	ModUnix int64
}

type PathProblem struct {
	Path string
	Err  string
}

type SelectionPlan struct {
	Paths          []string
	Files          []SelectedFile
	TotalBytes     int64
	ExcludedHidden int
	Problems       []PathProblem
}
