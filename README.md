# gifhub 
:chart_with_upwards_trend: Create GIFs from user's GitHub activity graph <a href="https://goreportcard.com/report/github.com/camilogarcialarotta/gifhub"><img align="right" src="https://goreportcard.com/badge/github.com/camilogarcialarotta/gifhub"></a>

<p align="center">
<img src="https://i.imgur.com/lbsTxaV.gif" width="300">
</p>

## Go Dependencies
No non-Go dependencies required!
- [fogleman/gg](https://github.com/fogleman/gg): 2D graphics library
- [urfave/cli](https://github.com/urfave/cli): CLI framework

## How to use
1. If your GitHub profile does not yet display activity overviews, [enable it](https://github.blog/changelog/2018-08-24-profile-activity-overview/)

2. Depending on your platform you can run `gifhub` in 2 ways:
- **Linux/OSX/Windows:** Build and run a Docker container with the app. Requires Docker
- **Linux:** Compile directly from source. Requires Golang and the 2 dependencies (fogleman/gg, urfave/cli)

Regardless of how you choose to run the app, clone the repository
  ```bash
  git clone https://github.com/CamiloGarciaLaRotta/gifhub.git
  cd gifhub
  ``` 

### Run via Docker
Build the image
```bash
docker build . -t gifhub
```

Create a directory of your choice to store the output GIF.  
Run the container with the created directory mounted to `/app/out`
```bash
mkdir out
docker run \
  -it \
   --rm \
  -v $(pwd)/out:/app/out \
  gifhub camilogarcialarotta
```

The application will generate a GIF named after the user inside `./out`  
For more information on available flags, run `gifhub --help`

### Run directly in your machine
Install the binary and its dependencies
```bash
# install fogleman/gg, urfave/cli and gifhub
go install ./...
```

Run the CLI with a GitHub handle
```bash
gifhub camilogarcialarotta
```

The application will generate a GIF named after the user inside `./out`  
For more information on available flags, run `gifhub --help`


## Credits
Special thanks to:
 - [bclindner](https://github.com/bclindner) for the name idea
 - [DestructiveReasoning](https://github.com/DestructiveReasoning) for math help and testing on Linux distros
 - [erickzhao](https://github.com/erickzhao) for testing on OSX
