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
	{Id: "staging", Name: "staging", Path: namespace + root + "staging/", IsFile: false},
	{Id: "staging_logs_file", Name: "staging logs file", Path: namespace + root + "staging/logs.json", IsFile: true, Content: "[]"},
	{Id: "commits", Name: "commits", Path: namespace + root + "commits/", IsFile: false},
	{Id: "branches", Name: "branches", Path: namespace + root + "branches/", IsFile: false},
	{Id: "default_branch_dir", Name: "main (default branch)", Path: namespace + root + "branches/main/", IsFile: false},
	{Id: "default_branch_commits_file", Name: "commits (default branch)", Path: namespace + root + "branches/main/commits.json", IsFile: true, Content: "[]"},
	{Id: "branches_metadata", Name: "metadata", Path: namespace + root + "branches/metadata.json", IsFile: true, Content: "{ \"default\": \"main\", \"current\": \"main\" }"},
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
