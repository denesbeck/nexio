package main

type DirEntry struct {
	Id      string
	Name    string
	Path    string
	IsFile  bool
	Content string
}

var root = ".nexio/"

var dirs = []DirEntry{
	{Id: "root", Name: "root", Path: namespace + root, IsFile: false},
	{Id: "objects", Name: "objects", Path: namespace + root + "objects/", IsFile: false},
	{Id: "config", Name: "config", Path: namespace + root + "config.json", IsFile: true, Content: "{ \"name\": \"\", \"email\": \"\" }"},
}

func GetDirs() []string {
	var paths []string
	for _, dir := range dirs {
		if !dir.IsFile {
			paths = append(paths, dir.Path)
		}
	}
	return paths
}

func GetFiles() []string {
	var paths []string
	for _, dir := range dirs {
		if dir.IsFile {
			paths = append(paths, dir.Path)
		}
	}
	return paths
}

func GetRoot() string {
	return GetDir("root")
}

func GetDir(id string) string {
	for _, d := range dirs {
		if d.Id == id {
			return d.Path
		}
	}
	return ""
}
