# Shopit API

## About The Project

Shopit API is a robust and feature-rich backend service for a modern e-commerce platform. It's built with Go and follows best practices for building scalable and maintainable web services.

This API provides a comprehensive set of endpoints to power a full-featured online store, including:

*   **Complete User Authentication:**
    *   Secure user registration and login with password hashing (bcrypt).
    *   Token-based authentication for protected routes.
    *   Password recovery mechanism with email-based token reset.
    *   Role-based access control, with distinct permissions for regular users and administrators.
    *   Full user profile management.

*   **Advanced Product Management:**
    *   CRUD operations for products.
    *   Support for product reviews and ratings.
    *   Image uploads for products, leveraging Cloudinary for cloud-based media management.
    *   Admin-only endpoints for managing the entire product catalog.

*   **Comprehensive Order Management:**
    *   Users can create new orders from products in the catalog.
    *   Users can view their order history.
    *   Admins can view and manage all orders in the system, including updating order status (e.g., from 'pending' to 'shipped').

*   **Secure Payment Processing:**
    *   Integration with Stripe for secure and reliable payment processing.

*   **And more:**
    *   Structured logging with Zap for better observability.
    *   Configuration management with Viper.
    *   A complete database schema with migrations.

The API is designed to be RESTful and easy to consume by any front-end client (web or mobile).


## Features

### User Features:
- User registration and login
- Product browsing, searching, and filtering
- Product details view with reviews
- Add to cart functionality
- Checkout process with shipping information and payment
- Order history and details
- User profile management (update profile, password)
- Forgot and reset password functionality

### Admin Features:
- Admin dashboard with a summary of sales, products, orders, and users
- Product management (create, update, delete products)
- Order management (view, process, and delete orders)
- User management (view, update, and delete users)
- View and delete product reviews

## API Endpoints

The base URL for all endpoints is `/api/v1`.

### Authentication

- `POST /auth/register`: Register a new user.
- `POST /auth/login`: Login a user.
- `GET /auth/logout/{token}`: Logout user.
- `GET /auth/me`: Get current user profile.
- `PUT /auth/me`: Update current user profile.
- `POST /auth/password/forgot`: Forgot password.
- `PUT /auth/password/reset/{token}`: Reset password.
- `PUT /auth/password/update`: Update password.

### Authentication (Admin)

- `GET /auth/admin/users`: Get all users.
- `GET /auth/admin/user/{id}`: Get user details by ID.
- `PUT /auth/admin/user/{id}`: Update user by ID.
- `DELETE /auth/admin/user/{id}`: Delete user by ID.

### Products

- `GET /product/products`: Get all products.
- `GET /product/product/{id}`: Get a product by ID.
- `PUT /product/review`: Create or update a product review.
- `GET /product/reviews`: Get all reviews for a product.
- `DELETE /product/reviews`: Delete a product review.

### Products (Admin)

- `POST /product/new`: Create a new product.
- `GET /product/admin/products`: Get all products.
- `PUT /product/admin/product/{id}`: Update a product.
- `DELETE /product/admin/product/{id}`: Delete a product.

### Orders

- `POST /orders/new`: Create a new order.
- `GET /orders/me`: Get current user's orders.
- `GET /orders/{id}`: Get an order by ID.

### Orders (Admin)

- `GET /orders/admin/orders`: Get all orders.
- `PUT /orders/admin/order/{id}`: Update an order's status.
- `DELETE /orders/admin/order/{id}`: Delete an order.

### Payment

- `POST /payment/process`: Process a payment.
- `GET /payment/stripeapi`: Get Stripe API key.

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
server:
  AppVersion: 1.0.0
  Port: 5000
  PprofPort: ":5555"
  Mode: Development
  JwtSecretKey: "your_jwt_secret_key"
  CookieName: "jwt-token"
  ReadTimeout: 5
  WriteTimeout: 5
  SSL: false
  CtxDefaultTimeout: 12
  CSRF: true
  Debug: false

logger:
  Development: true
  DisableCaller: false
  DisableStacktrace: false
  Encoding: "console"
  Level: "info"

postgres:
  Host: "localhost"
  Port: 5432
  User: "your_db_user"
  Password: "your_db_password"
  Dbname: "shopit"
  SSLMode: "disable"
  PgDriver: "pg"
  Url: "postgresql://user:password@host:port/dbname"

cookie:
  Name: "jwt-token"
  MaxAge: 86400
  Secure: false
  HttpOnly: true

stripe:
  Secret: "your_stripe_secret_key"
  Key: "your_stripe_publishable_key"

smtp:
  Host: "smtp.example.com"
  Port: 587
  Username: "your_smtp_username"
  Password: "your_smtp_password"

cloudinary:
  Name: "your_cloudinary_cloud_name"
  Key: "your_cloudinary_api_key"
  Secret: "your_cloudinary_api_secret"
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
