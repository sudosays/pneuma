package main

import (
	"encoding/json"
	"fmt"
	"github.com/gdamore/tcell/v2"
	libui "github.com/sudosays/pneuma/internal/ui"
	"github.com/sudosays/pneuma/pkg/data/hugo"
	"io/ioutil"
	//"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"
)

type PneumaConfig struct {
	Extension string     `json:"extension"`
	Editor    string     `json:"editor"`
	Sites     []HugoSite `json:"sites"`
}

// HugoSite contains the information for a hugo site listed in the config
type HugoSite struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var config PneumaConfig
var ui libui.PneumaUI

func init() {
	home, err := os.UserHomeDir()
	configPath := path.Join(home, ".config", "pneuma.json")
	check(err)
	config, err = readConfig(configPath)
	check(err)
	ui = libui.Init()
}

func main() {
	quitCmdEventKey := libui.CommandKey{
		Key:  tcell.KeyRune,
		Rune: 'q',
		Mod:  tcell.ModNone,
	}
	cmds := make(map[libui.CommandKey]libui.Command)
	cmds[quitCmdEventKey] = libui.Command{Callback: quit, Description: "Quit"}
	ui.SetCommands(cmds)

	var site hugo.Blog
	if len(config.Sites) > 1 {
		site = siteSelect()
	} else {
		site = hugo.Load(config.Sites[0].Path)
	}

	siteOverview(site)
	for {
		ui.Tick()
	}

}

func siteSelect() hugo.Blog {
	ui.Reset()
	ui.AddLabel(0, 0, "Sites from config:")

	headings := []string{"Choice", "Name", "Path"}

	var sitesList [][]string
	for i, site := range config.Sites {
		sitesList = append(sitesList, []string{fmt.Sprintf("%d", i+1), site.Name, site.Path})
	}
	ui.AddTable(0, 2, headings, sitesList)
	prompt := "Please choose a site (default=1): "
	ui.AddLabel(0, 4+len(sitesList), prompt)
	ui.MoveCursor(len(prompt), 3+len(sitesList))
	ui.Draw()

	siteSelect := getSelection(len(sitesList))

	site := hugo.Load(config.Sites[siteSelect].Path)
	return site
}

func getSelection(max int) int {
	selection := 0
	input := ui.WaitForInput()
	choice, err := strconv.Atoi(input)
	if err == nil {
		choice--
		if choice > 0 && choice <= max {
			selection = choice
		} else {
			check(err)
		}
	}
	return selection
}

func readConfig(path string) (PneumaConfig, error) {
	conf := PneumaConfig{}
	configFile, err := os.Open(path)
	check(err)
	byteValue, err := ioutil.ReadAll(configFile)
	check(err)
	fmt.Printf("%s", byteValue)
	json.Unmarshal(byteValue, &conf)
	fmt.Printf("Config is: %+v", conf)
	configFile.Close()
	return conf, nil
}

func startEditor(path string) {
	editorCmd := exec.Command(config.Editor, path)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	ui.Suspend()
	err := editorCmd.Run()
	ui.Resume()
	check(err)
}

func siteOverview(site hugo.Blog) {
	ui.Reset()
	posts := site.Posts
	headings, postList := genPostList(site)

	ui.AddLabel(0, 0, "Posts from the blog:")
	table := ui.AddTable(0, 1, headings, postList)
	prompt := "Select a post to edit: "
	ui.AddLabel(0, 4+len(postList), prompt)
	ui.Draw()

	ui.MoveCursor(len(prompt), 3+len(postList))

	editPost := func() {
		path := posts[table.Index].Path
		startEditor(path)
	}

	newPost := func() {
		prompt := "Enter a title for the new post:"
		ui.AddLabel(0, 4+len(postList), prompt)
		ui.MoveCursor(len(prompt), 4+len(postList))
		ui.Draw()
		title := ui.WaitForInput()
		if title != "" {
			filePath := site.NewPost(title)
			startEditor(filePath)
			posts = site.Posts
			headings, postList := genPostList(site)
			table.SetContent(headings, postList)
		}
	}

	deletePost := func() {
		if ui.Confirm("Delete post? [y/N]: ") {
			path := posts[table.Index].Path
			site.DeletePost(path)
			headings, postList := genPostList(site)
			table.SetContent(headings, postList)
		}
	}

	nextCmdEventKey := libui.CommandKey{Key: tcell.KeyRune, Rune: 'j', Mod: tcell.ModNone}
	prevCmdEventKey := libui.CommandKey{Key: tcell.KeyRune, Rune: 'k', Mod: tcell.ModNone}
	quitCmdEventKey := libui.CommandKey{Key: tcell.KeyRune, Rune: 'q', Mod: tcell.ModNone}
	enterCmdEventKey := libui.CommandKey{Key: tcell.KeyEnter, Rune: rune(13), Mod: tcell.ModNone}
	syncCmdEventKey := libui.CommandKey{Key: tcell.KeyRune, Rune: 's', Mod: tcell.ModNone}
	newCmdEventKey := libui.CommandKey{Key: tcell.KeyRune, Rune: 'n', Mod: tcell.ModNone}
	deleteCmdEventKey := libui.CommandKey{Key: tcell.KeyRune, Rune: 'd', Mod: tcell.ModNone}

	cmds := make(map[libui.CommandKey]libui.Command)
	cmds[quitCmdEventKey] = libui.Command{Callback: quit, Description: "Quit"}
	cmds[nextCmdEventKey] = libui.Command{Callback: table.NextItem, Description: "Next item"}
	cmds[prevCmdEventKey] = libui.Command{Callback: table.PreviousItem, Description: "Prev item"}
	cmds[enterCmdEventKey] = libui.Command{Callback: editPost, Description: "Edit post"}
	cmds[syncCmdEventKey] = libui.Command{Callback: site.Synchronise, Description: "Sync"}
	cmds[newCmdEventKey] = libui.Command{Callback: newPost, Description: "Create a new post"}
	cmds[deleteCmdEventKey] = libui.Command{Callback: deletePost, Description: "Delete a post"}

	ui.SetCommands(cmds)
}

func quit() {
	ui.Close()
}

func genPostList(blog hugo.Blog) ([]string, [][]string) {

	posts := blog.Posts
	headings := []string{"#", "Date", "Title", "Draft"}
	var postList [][]string

	for i, post := range posts {
		draftStatus := "False"
		if post.Draft {
			draftStatus = "True"
		}
		datetime, _ := time.Parse(time.RFC3339, post.Date)
		date := datetime.Format("2006/01/02")
		postList = append(postList, []string{fmt.Sprintf("%d", i+1), date, post.Title, draftStatus})
	}

	return headings, postList
}
