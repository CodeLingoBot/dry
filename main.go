package main

import (
	"fmt"
	"os"
	"time"

	"net/http"
	_ "net/http/pprof"

	"github.com/jessevdk/go-flags"
	"github.com/moncho/dry/app"
	"github.com/moncho/dry/appui"
	"github.com/moncho/dry/docker"
	"github.com/moncho/dry/ui"
	"github.com/moncho/dry/version"
	log "github.com/sirupsen/logrus"
)

const (
	banner = `     _
    | |
  __| |  ____  _   _
 / _  | / ___)| | | |
( (_| || |    | |_| |
 \____||_|     \__  |
               (____/

`
	cheese     = "<white>made with ♥ (and go) by</> <blue>moncho</>"
	connecting = "ŏ Trying to connect to the Docker Host ŏ"
)

var loadMessage = []string{docker.Whale0,
	docker.Whale1,
	docker.Whale2,
	docker.Whale3,
	docker.Whale4,
	docker.Whale5,
	docker.Whale6,
	docker.Whale7,
	docker.Whale}

//dryOptions represents command line flags variables
type dryOptions struct {
	Description bool `short:"d" long:"description" description:"Dry description"`
	MonitorMode bool `short:"m" long:"monitor" description:"Starts dry in monitor mode"`
	// enable profiling
	Profile bool `short:"p" long:"profile" description:"Enable profiling"`
	Version bool `short:"v" long:"version" description:"Dry version"`
	//Docker-related properties
	DockerHost       string `short:"H" long:"docker_host" description:"Docker Host"`
	DockerCertPath   string `short:"c" long:"docker_certpath" description:"Docker cert path"`
	DockerTLSVerifiy string `short:"t" long:"docker_tls" description:"Docker TLS verify"`
	//Whale
	Whale uint `short:"w" long:"whale" description:"Show whale for w seconds"`
}

func newDockerEnv(opts dryOptions) *docker.Env {
	dockerEnv := docker.NewEnv()
	if opts.DockerHost == "" {
		if os.Getenv("DOCKER_HOST") == "" {
			log.Printf(
				"No DOCKER_HOST env variable found and no Host parameter was given, connecting to %s",
				docker.DefaultDockerHost)
			dockerEnv.DockerHost = docker.DefaultDockerHost
		} else {
			dockerEnv.DockerHost = os.Getenv("DOCKER_HOST")
			dockerEnv.DockerTLSVerify = docker.GetBool(os.Getenv("DOCKER_TLS_VERIFY"))
			dockerEnv.DockerCertPath = os.Getenv("DOCKER_CERT_PATH")
		}
	} else {
		dockerEnv.DockerHost = opts.DockerHost
		dockerEnv.DockerTLSVerify = docker.GetBool(opts.DockerTLSVerifiy)
		dockerEnv.DockerCertPath = opts.DockerCertPath
	}
	return dockerEnv
}

func showLoadingScreen(screen *ui.Screen, dockerEnv *docker.Env) chan<- struct{} {
	screen.Clear()
	midscreen := ui.ActiveScreen.Dimensions.Width / 2
	height := ui.ActiveScreen.Dimensions.Height
	screen.RenderAtColumn(midscreen-len(connecting)/2, 1, ui.White(connecting))
	screen.RenderLine(2, height-2, fmt.Sprintf("<blue>Dry Version:</> %s", ui.White(version.VERSION)))
	if dockerEnv != nil {
		screen.RenderLine(2, height-1, fmt.Sprintf("<blue>Docker Host:</> %s", ui.White(dockerEnv.DockerHost)))
	} else {
		screen.RenderLine(2, height-1, ui.White("No Docker host"))
	}

	stop := make(chan struct{})

	//20 is a safe aproximation for the length of interpreted characters from the message
	screen.RenderLine(ui.ActiveScreen.Dimensions.Width-len(cheese)+20, height-1, cheese)
	screen.Flush()
	go func() {
		rotorPos := 0
		forward := true
		ticker := time.NewTicker(250 * time.Millisecond)

		for {
			select {
			case <-ticker.C:
				loadingMessage := loadMessage[rotorPos]
				screen.RenderAtColumn(midscreen-19, height/2-6, ui.Cyan(loadingMessage))
				screen.Flush()
				if rotorPos == len(loadMessage)-1 {
					forward = false
				} else if rotorPos == 0 {
					forward = true
				}
				if forward {
					rotorPos++
				} else {
					rotorPos--
				}

			case <-stop:
				return
			}
		}
	}()
	return stop
}
func main() {
	running := false

	defer func() {
		if r := recover(); r != nil {
			log.Fatalf(
				"Dry panicked: %d", r)
			log.Print("Bye")
			os.Exit(1)
		} else if running {
			log.Print("Bye")
		}
	}()
	// parse flags
	var opts dryOptions
	var parser = flags.NewParser(&opts, flags.Default)
	_, err := parser.Parse()
	if err != nil {
		flagError := err.(*flags.Error)
		if flagError.Type == flags.ErrHelp {
			return
		}
		if flagError.Type == flags.ErrUnknownFlag {
			log.Print("Use --help to view all available options.")
			return
		}
		log.Printf("Error parsing flags: %s", err)
		return
	}
	if opts.Description {
		fmt.Print(app.ShortHelp)
		return
	}
	if opts.Version {
		fmt.Printf("dry version %s, build %s", version.VERSION, version.GITCOMMIT)
		return
	}
	log.Print("Starting dry")
	dockerEnv := newDockerEnv(opts)

	// Start profiling (if required)
	if opts.Profile {
		go func() {
			log.Fatal(http.ListenAndServe("localhost:6060", nil))
		}()
	}
	screen, err := ui.NewScreen(appui.DryTheme)
	if err != nil {
		log.Printf("Dry error: %s", err)
		return
	}

	running = true

	start := time.Now()
	stopLoadScreen := showLoadingScreen(screen, dockerEnv)

	dry, err := app.NewDry(screen, dockerEnv)

	//show whale to bablat
	if opts.Whale > 0 {
		showWale, _ := time.ParseDuration(fmt.Sprintf("%ds", opts.Whale))
		time.Sleep(showWale - time.Since(start))
	}
	//dry has loaded, stopping the loading screen
	close(stopLoadScreen)
	if err == nil {
		if opts.MonitorMode {
			dry.ViewMode(app.Monitor)
		}
		app.RenderLoop(dry, screen)
	} else {
		defer log.Printf("Dry error: %s", err)
	}
	screen.Close()
}
