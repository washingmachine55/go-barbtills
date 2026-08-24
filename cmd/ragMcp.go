/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	l "barbtils/internal/logger"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// ragMcpCmd represents the ragMcp command
var ragMcpCmd = &cobra.Command{
	Use:     "ragMcp",
	Short:   "ramcp",
	Long:    `A RAG and MCP to be used with an AI Agent`,
	Example: `fmt.Fprintf(out, "Usage: %s <client|server> [-proto <http|https>] [-port <port] [-host <host>]\n\n", os.Args[0])`,
	Run: func(cmd *cobra.Command, args []string) {
		if cmd.Flag("start") != nil {
			// RagMCPServer()
			serveMcp()
		}
	},
}

func init() {
	rootCmd.AddCommand(ragMcpCmd)

	ragMcpCmd.Flags().BoolP("start", "s", true, "Start MCP Server with RAG database connection")
	ragMcpCmd.Usage()
}

func GetRagDbURL() string {
	initConfig()
	ragDbURL := viper.GetString("RAG_DB_URL")
	if ragDbURL == "" {
		l.Logger.Fatal("RAG_DB_URL is not configured — set it in your barbtils.toml or environment")
	}
	return ragDbURL
}

func openRagDb() (*sql.DB, error) {
	db, err := sql.Open("postgres", GetRagDbURL())
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to reach database: %w", err)
	}
	return db, nil
}

func insertChunk(db *sql.DB, req ragInsertRequest) (sql.Result, error) {
	result, err := db.Exec(`INSERT INTO document_chunks (file_path, content, embedding)
		VALUES ($1, $2, $3;`, req.FilePath, req.Content, req.Embedding)
	if err != nil {
		return nil, errors.New("Query failed to insert into RAG DB")
	}
	return result, nil
}

type ragInsertRequest struct {
	FilePath  string  `json:"filePath"`
	Content   string  `json:"content"`
	Embedding []int32 `json:"embedding"`
}

func verctorRag(ctx context.Context, req *mcp.CallToolRequest, params *ragInsertRequest) (*mcp.CallToolResult, any, error) {
	var jsonData json.RawMessage = req.Params.Arguments
	err := req.Params.Arguments.UnmarshalJSON(jsonData)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to Unmarshal json data: %s", err)
	}

	content := params.Content
	if content == "" {
		return nil, nil, errors.New("Content not provided")
	}

	filePath := params.FilePath
	if filePath == "" {
		return nil, nil, errors.New("FilePath not provided")
	}

	embedding := params.Embedding
	if len(embedding) <= 0 {
		return nil, nil, errors.New("No Embeddings provided")
	}

	response := fmt.Sprintf("The current time in %s is %s",
		cityNames[city],
		now.Format(time.RFC3339))

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: response},
		},
	}, nil, nil
}

func RagMCPServer() {
	db, err := openRagDb()
	if err != nil {
		l.Logger.Fatal("Unable to connect to RAG DB — set it in your barbtils.toml or environment")
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	http.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req ragInsertRequest
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		result, err := insertChunk(db, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if result != nil {
			err := errors.New("No result obtained from insert query")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"status":"success"}`))
	})
	l.Info("[RAG-MCP]", "message", "Server running on port 42069")
	if err := http.ListenAndServe(":42069", nil); err != nil {
		l.Fatal(err)
	}
}

func createLoggingMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(
			ctx context.Context,
			method string,
			req mcp.Request,
		) (mcp.Result, error) {
			start := time.Now()
			sessionID := req.GetSession().ID()

			// Log request details.
			log.Printf("[REQUEST] Session: %s | Method: %s",
				sessionID,
				method)

			// Call the actual handler.
			result, err := next(ctx, method, req)

			// Log response details.
			duration := time.Since(start)

			if err != nil {
				log.Printf("[RESPONSE] Session: %s | Method: %s | Status: ERROR | Duration: %v | Error: %v",
					sessionID,
					method,
					duration,
					err)
			} else {
				log.Printf("[RESPONSE] Session: %s | Method: %s | Status: OK | Duration: %v",
					sessionID,
					method,
					duration)
			}

			return result, err
		}
	}
}

var (
	host  = flag.String("host", "localhost", "host to connect to/listen on")
	port  = flag.Int("port", 42069, "port number to connect to/listen on")
	proto = flag.String("proto", "http", "if set, use as proto:// part of URL (ignored for server)")
)

func runServer(url string) {
	// Create an MCP server.
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "time-server",
		Version: "1.0.0",
	}, nil)

	// Add MCP-level logging middleware.
	server.AddReceivingMiddleware(createLoggingMiddleware())

	// Add the cityTime tool.
	mcp.AddTool(server, &mcp.Tool{
		Name:        "RAG",
		Description: "Retrieval-Augmented Generation using pgvector",
	}, verctorRag)

	// Create the streamable HTTP handler.
	handler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, nil)

	log.Printf("MCP server listening on %s", url)
	log.Printf("Available tool: RAG")

	// Start the HTTP server.
	if err := http.ListenAndServe(url, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func runClient(url string) {
	ctx := context.Background()

	// Create the URL for the server.
	log.Printf("Connecting to MCP server at %s", url)

	// Create an MCP client.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "time-client",
		Version: "1.0.0",
	}, nil)

	// Connect to the server.
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: url}, nil)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer session.Close()

	log.Printf("Connected to server (session ID: %s)", session.ID())

	// First, list available tools.
	log.Println("Listing available tools...")
	toolsResult, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Fatalf("Failed to list tools: %v", err)
	}

	for _, tool := range toolsResult.Tools {
		log.Printf("  - %s: %s\n", tool.Name, tool.Description)
	}

	// Call the cityTime tool for each city.
	cities := []string{"nyc", "sf", "boston"}

	log.Println("Getting time for each city...")
	for _, city := range cities {
		// Call the tool.
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name: "cityTime",
			Arguments: map[string]any{
				"city": city,
			},
		})
		if err != nil {
			log.Printf("Failed to get time for %s: %v\n", city, err)
			continue
		}

		// Print the result.
		for _, content := range result.Content {
			if textContent, ok := content.(*mcp.TextContent); ok {
				log.Printf("  %s", textContent.Text)
			}
		}
	}

	log.Println("Client completed successfully")
}
