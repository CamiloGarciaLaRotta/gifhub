# activitygiffer
:chart_with_upwards_trend: Create GIFs from user's GitHub activity graph

## :construction: **WIP** :construction:

<img src="https://i.imgur.com/3Y7VLb7.gif" width="300">

## Dependencies
- [ImageMagick](https://www.imagemagick.org/): Convert SVG :arrow_right: JPG :arrow_right: GIF
- [urfave/cli](https://github.com/urfave/cli): CLI framework

## How to use
If you are working on Windows or don't want to install the dependencies, run the application via Docker.

Regardless of how you choose to run the app, clone the repository
  ```bash
  git clone https://github.com/CamiloGarciaLaRotta/activitygiffer.git
  cd activitygiffer
  ``` 

### Run via Docker
Build the image
```bash
docker build . -t activitygiffer
```

Create a directory of your choice to store the output GIF.  
Run the container with the created directory mounted to `/app/out`
```bash
mkdir out
docker run \
  -it \
   --rm \
  -v $(pwd)/out:/app/out \
  activitygiffer -y 2016,2017,2018,2019 camilogarcialarotta
```

The application will generate a GIF named after the user inside `./out`

### Run directly in your machine
Install the binary and its dependencies
```bash
# install ImageMagick 
sudo dnf install ImageMagick     # Fedora
sudo apt-get install imagemagick # Debian

# install urfave/cli
go install ./...
```

Run the CLI with a GitHub handle
```bash
activitygiffer -y 2016,2018,2019 camilogarcialarotta
```

The application will generate a GIF named after the user inside `./out`
