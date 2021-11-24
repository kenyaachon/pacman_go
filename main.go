package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"github.com/danicat/simpleansi"
)


var maze []string


type sprite struct {
	row int
	column int
	startRow int
	startColumn int
}

type Config struct {
	Player string `json:"player"`
	Ghost string `json:"ghost"`
	Wall string `json:"wall"`
	Dot string `json:"dot"`
	Pill string `json:"pill"`
	Death string `json:"death"`
	Space string `json:"space"`
	UseEmoji bool `json:"use_emoji"`
	GhostBlue string `json:"ghost_blue"`
	PillDurationSecs time.Duration `json:"pill_duration_secs"`
}

var configuration Config
var player sprite


var score int
var numDots int
var lives = 3

var (
	configFile = flag.String("config-file", "config.json", "path to custom configruation file")
	mazeFile = flag.String("maze-file", "maze01.txt", "path to a custom maze file")
)

type GhostStatus string

type ghostType struct {
	position sprite
	status GhostStatus
}

const (
	GhostStatusNormal GhostStatus = "Normal"
	GhostStatusBlue GhostStatus = "Blue"
)

var ghosts []*ghostType


var pillTimer *time.Timer
var pillMutex sync.Mutex
var ghostsStatusMutex sync.RWMutex
func loadConfig(file string) error {
	//Parses the JSON file to load the game configuration
	f, err := os.Open(file)
	if err != nil {
		return err
	}

	defer f.Close()
	decoder := json.NewDecoder(f)
	err = decoder.Decode(&configuration)


	if err != nil {
		return err
	}
	return nil
}
func loadMaze(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		maze = append(maze, line)
	}


	//traverse each character of the maze and create a new player 
	//when it locates a `P`
	for row, line := range maze {
		for column, char := range line {
			switch char {
			case 'P':
				player = sprite{row, column, row, column}
			case 'G':
				ghosts = append(ghosts, &ghostType{sprite{row, column, row, column}, GhostStatusNormal})
			case '.':
				numDots++
			}
		}
	}
	return nil
}

func initialise() {
	//enabling cbreak mode
	cbTerm := exec.Command("stty", "cbreak", "-echo")
	cbTerm.Stdin = os.Stdin
	
	err := cbTerm.Run()
	if err != nil {
		log.Fatalln("unable to activate cbrerak mode: ", err)
	}
}

func cleanup() {
	//restoring cooked mode
	cookedTerm := exec.Command("stty", "-cbreak", "echo")
	cookedTerm.Stdin = os.Stdin
	
	err := cookedTerm.Run()
	if err != nil {
		log.Fatalln("unable to restore cooked mode:", err)
	}
}

func readInput() (string, error) {
	buffer := make([]byte, 100)

	count, err := os.Stdin.Read(buffer)
	if err != nil {
		return "", err
	}

	if count == 1 && buffer[0] == 0x1b {
		return "ESC", nil
	} else if count >= 3 {
		//handling input for arrow keys
		if buffer[0] == 0x1b && buffer[1] == '['  {
			switch buffer[2] {
			case 'A':
				return "UP", nil
			case 'B':
				return "DOWN", nil
			case 'C':
				return "RIGHT", nil
			case 'D':
				return "LEFT", nil
			}
		}
	}

	return "", nil


}


func makeMove(oldRow int, oldColumn int, direction string) (newRow, newColumn int) {
	newRow, newColumn = oldRow, oldColumn

	switch direction {
	case "UP":
		newRow -= 1
		if newRow < 0 {
			newRow = len(maze) - 1
		}
	case "DOWN":
		newRow += 1
		if newRow == len(maze) {
			newRow = 0
		}
	case "RIGHT":
		newColumn += 1
		if newColumn == len(maze[0]) {
			newColumn = 0
		}
	case "LEFT":
		newColumn -= 1
		if newColumn < 0 {
			newColumn = len(maze[0]) - 1
		}
	}

	if maze[newRow][newColumn] == '#' {
		newRow = oldRow
		newColumn = oldColumn
	}
	return
}

func updateGhosts(ghosts []*ghostType, ghostStatus GhostStatus) {
	ghostsStatusMutex.Lock()
	defer ghostsStatusMutex.Unlock()
	for _, ghost := range ghosts {
		ghost.status = ghostStatus
	}
}
func processPill() {
	// for _, g := range ghosts {
	// 	g.status = GhostStatusBlue
	// }

	// pillTimer = time.NewTimer(time.Second * configuration.PillDurationSecs)
	// <-pillTimer.C
	// for _, g := range ghosts {
	// 	g.status = GhostStatusNormal
	// }

	//supports simulaaneous pill swallowing
	pillMutex.Lock()
	updateGhosts(ghosts, GhostStatusBlue)
	if pillTimer != nil {
		pillTimer.Stop()
	}

	pillTimer = time.NewTimer(time.Second * configuration.PillDurationSecs)
	pillMutex.Unlock()
	<-pillTimer.C
	pillMutex.Lock()
	pillTimer.Stop()
	updateGhosts(ghosts, GhostStatusNormal)
	pillMutex.Unlock()
}

func movePlayer(direction string) {
	player.row, player.column = makeMove(player.row, player.column, direction)

	//removeDot from the maze
	removeDot := func(row, column int) {
		maze[row] = maze[row][0:column] + " " + maze[row][column+1:]

	}
	switch maze[player.row][player.column] {
	case '.':
		numDots--
		score++
		removeDot(player.row, player.column)
	case 'X':
		score += 10
		removeDot(player.row, player.column)
		go processPill()
	}
}

func moveCursor(row, column int) {
	if configuration.UseEmoji{
		simpleansi.MoveCursor(row, column*2)
	} else {
		simpleansi.MoveCursor(row, column)
	}
}

//concatenate the correct number of player emojis based on lives
func getLivesAsEmoji() string {
	buffer := bytes.Buffer{}
	for i := lives; i > 0; i-- {
		buffer.WriteString(configuration.Player)
	}
	return buffer.String()
}
func printScreen() {
	simpleansi.ClearScreen()
	for _, line := range maze {
		for _, chr := range line {
			switch chr {
			case '#':
				fmt.Print(simpleansi.WithBlueBackground(configuration.Wall))
			case '.':
				fmt.Printf(configuration.Dot)
			default:
				fmt.Print(configuration.Space)
			}
		}

		fmt.Println()
	}

	//printing the player
	moveCursor(player.row, player.column)
	fmt.Print(configuration.Player)

	//printing the ghosts
	for _, g := range ghosts {
		moveCursor(g.position.row, g.position.column)
		if g.status == GhostStatusNormal {
			fmt.Printf(configuration.Ghost)
		} else if g.status == GhostStatusBlue {
			fmt.Printf(configuration.GhostBlue)
		}
	}

	//Move cursor outside of maze drawing area
	moveCursor(len(maze)+1, 0)

	//print score and lives

	//converts lives from int to string
	livesRemaining := strconv.Itoa(lives)
	if configuration.UseEmoji {
		livesRemaining = getLivesAsEmoji()
	}

	fmt.Println("Score:", score, "\tLives:", livesRemaining)
}


func drawDirecton() string {
	//randomly position the ghosts on the map
	direction := rand.Intn(4)
	move := map[int]string {
		0: "UP",
		1: "DOWN",
		2: "RIGHT",
		3: "LEFT",
	}
	return move[direction]
}

func moveGhosts() {
	for _, ghost := range ghosts {
		direction := drawDirecton()
		ghost.position.row, ghost.position.column = makeMove(ghost.position.row, ghost.position.column, direction)
	}
}

func main() {
	flag.Parse()

	//initialize game
	initialise()
	defer cleanup()

	//load resources
	err := loadMaze(*mazeFile)
	if err != nil {
		log.Println("failed to load maze:", err)
		return
	}


	//load configuration
	err = loadConfig(*configFile)
	if err != nil {
		log.Println("failed to load configuration:", err)
		return
	}

	//game loop
	//process input asynchronuusly
	inputChannel := make(chan string)
	go func(ch chan<- string) {
		for {
			input, err := readInput()
			if err != nil {
				log.Println("error reading input:", err)
				ch <- "ESC"
			}
			ch <-input
		}
	}(inputChannel)

	for {
		//update screen
		printScreen()

		

		//process movement
		select {
		//reading from the inputChannel
		case input := <-inputChannel:
			if input == "ESC" {
				lives = 0
			}
			movePlayer(input)
		default:
		}
		moveGhosts()

		//process collisions
		for _, ghost := range ghosts {
			if player.row == ghost.position.row && player.column == ghost.position.column {
				lives --
				if lives > 0 {
					moveCursor(player.row, player.column)
					fmt.Print(configuration.Death)
					moveCursor(len(maze)+2, 0)
					//dramatica pause before resetting player position
					time.Sleep(1000 *time.Millisecond)
					player.row, player.column = player.startRow, player.startColumn
				}
			}
		}

		if numDots == 0 || lives <= 0 {
			if lives <= 0 {
				//Game Over Sprite
				moveCursor(player.row, player.column)
				fmt.Print(configuration.Death)
				moveCursor(len(maze)+2, 0)
			}
			break
		}

		//check game over
		fmt.Println("Hello, Pac, Go!")
		
		

		//repeat
		time.Sleep(200*time.Millisecond)
	}
}


