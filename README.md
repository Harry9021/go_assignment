Here’s a clean and professional `README.md` based on your provided setup and configuration instructions:

---

# Bidirectional ClickHouse & Flat File Data Ingestion Tool

This project enables seamless data ingestion and transfer between ClickHouse and flat file formats like CSV, with a simple web interface and powerful backend capabilities.

## 🚀 Features

- Bidirectional data flow between ClickHouse and CSV files.
- Intuitive UI for testing and configuration.
- Easy setup and quick testing with preloaded sample data.

---

## 🧠 Technologies Used

- **Backend**: Golang
- **Frontend**: React (with npm)
- **Database**: ClickHouse (Cloud-hosted)

---

## ⚙️ Setup Instructions

### 🔧 Backend

1. Make sure you have Go installed.
2. Navigate to the backend directory.
3. Run the following command:

```bash
go run main.go clickhouse.go flatfile.go config.go handlers.go
```

---

### 🌐 Frontend

1. Navigate to the frontend directory.
2. Run the following commands:

```bash
npm install
npm start
```

---

## 🧪 Testing

On the UI, for proper testing, use the following credentials:

```
Host:      da2nakjy9k.ap-south-1.aws.clickhouse.cloud  
Port:      9440  
Database:  default  
Username:  default  
JWT Token: JuGG4kUe~0tZe
```

> ✅ Some data tables have already been loaded into the `default` database for testing purposes.

You can test the ingestion feature by interchanging the data source:
- **From CSV to ClickHouse**
- **From ClickHouse to CSV**

---

## 📂 File Overview

- `main.go`: Entry point of the backend.
- `clickhouse.go`: Handles ClickHouse database operations.
- `flatfile.go`: Manages flat file reading/writing.
- `config.go`: Stores configuration logic.
- `handlers.go`: API route handlers.
