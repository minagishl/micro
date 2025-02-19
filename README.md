# Micro

`micro` is a Go application that monitors seismic activity by connecting to a real-time WebSocket API, processes earthquake data, and posts formatted alerts to Discord via webhooks.
This project is inspired by the original [micro](https://github.com/evacuate/micro) project and re-implements its core functionality in Go.

## Features

- Connects to a WebSocket API to receive earthquake data.
- Processes seismic intensity and event codes.
- Posts formatted earthquake information to a service using the Discord Webhook.
- Automatically reconnects using exponential backoff if connection issues occur.

## Getting Started

### Prerequisites

- Go (Version 1.20 or later)
- Discord Webhook (A valid Discord webhook URL)

### Installation

1. Clone the repository:

   ```bash
   git clone https://github.com/minagishl/micro.git
   cd micro
   ```

2. Install dependencies:

   Use Go modules to manage dependencies automatically:

   ```bash
   go mod tidy
   ```

### Configuration

1. Copy the example configuration file:

   ```bash
   cp .env.example .env
   ```

2. Edit the `.env` file

### Build and Run

- **To build the project:**

  ```bash
  go build -o micro
  ```

  Run the compiled binary:

  ```bash
  ./micro
  ```

- **To run directly from source:**

  ```bash
  go run .
  ```

## Contributing

Contributions are welcome!
If you find a bug or have suggestions for improvements, please open an issue or submit a pull request.

## Author

- Minagishl ([@minagishl](https://github.com/minagishl))

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
