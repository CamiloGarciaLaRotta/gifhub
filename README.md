# activitygiffer
:chart_with_upwards_trend: Create GIFs from user's GitHub activity graph

:construction:**WIP**:construction:

## Usage
Clone the repository
  ```bash
  git clone https://github.com/CamiloGarciaLaRotta/activitygiffer.git
  cd activitygiffer
  ```

Install the binary and its dependencies
```bash
go install ./...
```

Run the CLI with a GitHub handle
```bash
activitygiffer camilogarcialarotta
```

The application will generate an SVG file of the same name as the user handle

## TODO
- Scrape all available years from a user's homepage
- Add polygon to SVG
- Rasterize SVGs as GIFs
