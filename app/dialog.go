package main

type FileDialogRequest struct {
	Title            string `json:"title"`
	FilterName       string `json:"filterName"`
	FilterGlob       string `json:"filterGlob"`
	Directory        bool   `json:"directory"`
	DefaultDirectory string `json:"defaultDirectory"`
	DefaultFilename  string `json:"defaultFilename"`
}
