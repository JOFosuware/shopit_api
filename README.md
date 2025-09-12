# Shopit API

Shopit API is a backend service for an e-commerce platform. It provides endpoints for managing products, users, orders, and payments.

## Features

- **User Authentication**: User registration and login.
- **Product Management**: CRUD operations for products.
- **Order Management**: Create, retrieve, and update orders.
- **Payment Processing**: Integration with Stripe for payments.
- **Image Uploads**: Cloudinary integration for product image uploads.

## API Endpoints

### Authentication

- `POST /auth/register`: Register a new user.
- `POST /auth/login`: Login a user.
- `GET /auth/me`: Get the currently authenticated user.
- `PUT /auth/me`: Update the currently authenticated user.

### Products

- `GET /products`: Get all products.
- `GET /products/{id}`: Get a single product by ID.
- `POST /products`: Create a new product (authentication required).
- `PUT /products/{id}`: Update a product (authentication required).
- `DELETE /products/{id}`: Delete a product (authentication required).

### Orders

- `GET /orders`: Get all orders for the authenticated user.
- `GET /orders/{id}`: Get a single order by ID.
- `POST /orders`: Create a new order.
- `PUT /orders/{id}`: Update an order.

### Payment

- `POST /payment/stripe`: Process a payment with Stripe.

## Technologies Used

- **Go**: The primary programming language.
- **PostgreSQL**: The database for storing data.
- **Chi**: A lightweight, idiomatic and composable router for building Go HTTP services.
- **Stripe**: For payment processing.
- **Cloudinary**: For image hosting.
- **Zap**: For logging.
- **Viper**: For configuration management.

## Getting Started

### Prerequisites

- Go (version 1.21.3 or newer)
- PostgreSQL
- Docker (optional)

### Installation

1.  **Clone the repository:**

    ```sh
    git clone https://github.com/jofosuware/shopit_api.git
    cd shopit_api
    ```

2.  **Install dependencies:**

    ```sh
    go mod tidy
    ```

3.  **Set up environment variables:**

    Create a `config-local.yml` file in the `config` directory. You can copy the structure from `config/config.go`.

    ```yaml
    app:
      name: "shopit"
      version: "1.0.0"
    db:
      dsn: "host=localhost port=5432 user=user password=password dbname=shopit sslmode=disable"
    server:
      port: "8080"
      read_timeout: 5s
      write_timeout: 10s
      idle_timeout: 15s
      # ... other server configs
    token:
      secret: "your-secret"
      # ... other token configs
    mail:
      host: "smtp.mailtrap.io"
      # ... other mail configs
    cloud:
      cloud_name: "your-cloud-name"
      api_key: "your-api-key"
      api_secret: "your-api-secret"
      upload_folder: "shopit"
    stripe:
      secret: "your-stripe-secret"
      key: "your-stripe-key"
    ```

4.  **Run database migrations:**

    You will need a migration tool that works with your SQL files in the `migrations` directory.

### Running the Application

You can use the provided `Makefile` to run the application.

-   **Start the application:**

    ```sh
    make start
    ```

-   **Stop the application:**

    ```sh
    make stop
    ```

-   **Build the application:**

    ```sh
    make build
    ```

### Running Tests

To run the tests for this project, you will need to have Go installed and configured on your system. Once you have that set up, you can run the following command in the root of the project directory:

```sh
go test ./...
```

## Project Structure

The project follows a standard Go project layout:

-   `cmd/api`: Main application entry point.
-   `internal`: Private application and library code.
    -   `auth`: Authentication logic.
    -   `orders`: Order management logic.
    -   `products`: Product management logic.
    -   `payment`: Payment processing logic.
    -   `models`: Database models.
    -   `server`: HTTP server and routing.
-   `pkg`: Public library code.
    -   `bcrypt`: Password hashing.
    -   `cloudinary`: Cloudinary client.
    -   `logger`: Logging setup.
    -   `mailer`: Email sending.
    -   `...` and other utility packages.
-   `config`: Configuration files and logic.
-   `migrations`: Database migration files.
-   `Makefile`: Commands for building, running, and stopping the application.
