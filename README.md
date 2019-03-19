# activitygiffer
:chart_with_upwards_trend: Create GIFs from user's GitHub activity graph

## :construction:**WIP**:construction:

## Dependencies
- [ImageMagick](https://www.imagemagick.org/): Convert SVG :arrow_right: JPG :arrow_right: GIF
- [urfave/cli](https://github.com/urfave/cli): CLI framework

## Usage
Clone the repository
  ```bash
  git clone https://github.com/CamiloGarciaLaRotta/activitygiffer.git
  cd activitygiffer
  ```

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

The application will generate a GIF named after the user

<img src="https://i.imgur.com/3Y7VLb7.gif" width="300">
