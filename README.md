# gifhub 
:chart_with_upwards_trend: Create GIFs from user's GitHub activity graph <a href="https://goreportcard.com/report/github.com/camilogarcialarotta/gifhub"><img align="right" src="https://goreportcard.com/badge/github.com/camilogarcialarotta/gifhub"></a>

<p align="center">
<img src="https://user-images.githubusercontent.com/17187770/80809567-eeba6a80-8b8f-11ea-8a91-987fbfab002d.gif" width="300">
</p>

## Go Dependencies
No non-Go dependencies required, so the binary is cross-platform!
- [fogleman/gg](https://github.com/fogleman/gg): 2D graphics library
- [urfave/cli](https://github.com/urfave/cli): CLI framework

## How to use
1. If your GitHub profile does not yet display activity overviews, [enable it](https://github.blog/changelog/2018-08-24-profile-activity-overview/)

2. You can run `gifhub` in 2 ways:

### If you already have Go

```bash
# Install the binary and its dependencies
go get github.com/camilogarcialarotta/gifhub

# Then run the CLI with a GitHub handle
gifhub camilogarcialarotta
```

The application will generate a GIF named after the user inside `./out`  
For more information on available flags, run `gifhub --help`

### If you don't want to install Go
Build and run a Docker container with the app. Requires Docker.

First, clone the repository and build the image
```bash 
git clone https://github.com/CamiloGarciaLaRotta/gifhub.git
cd gifhub

docker build . -t gifhub
```

Then, create a directory of your choice to store the output GIF.  
Run the container with the created directory mounted to `/app/out`
```bash
mkdir out
docker run -t \
  -v $(pwd)/out:/app/out \
  gifhub camilogarcialarotta
```

The application will generate a GIF named after the user inside `./out`  
For more information on available flags, run `gifhub --help`

## Credits
Special thanks to:
 - [bclindner](https://github.com/bclindner) for the name idea
 - [DestructiveReasoning](https://github.com/DestructiveReasoning) for math help and testing on Linux distros
 - [erickzhao](https://github.com/erickzhao) for testing on OSX
