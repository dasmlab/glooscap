package nanabush

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Client is a gRPC client for communicating with the Nanabush translation service.
type Client struct {
	conn   *grpc.ClientConn
	addr   string
	secure bool
	// TODO: Add generated client stub once proto is compiled:
	// client nanabushv1.TranslationServiceClient
}

// Config contains configuration for the Nanabush client.
type Config struct {
	// Address is the gRPC server address (e.g., "nanabush-service.nanabush.svc:50051")
	Address string
	// Secure enables TLS/mTLS (default: false for now)
	Secure bool
	// TLSCertPath is the path to the client TLS certificate (for mTLS)
	TLSCertPath string
	// TLSKeyPath is the path to the client TLS private key
	TLSKeyPath string
	// TLSCAPath is the path to the CA certificate for server verification
	TLSCAPath string
	// Timeout is the connection timeout
	Timeout time.Duration
}

// NewClient creates a new Nanabush gRPC client.
func NewClient(cfg Config) (*Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("nanabush: address is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	var opts []grpc.DialOption

	// Configure TLS/mTLS
	if cfg.Secure {
		// TODO: Load TLS credentials from cfg.TLSCertPath, cfg.TLSKeyPath, cfg.TLSCAPath
		// For now, use insecure for development
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	opts = append(opts, grpc.WithTimeout(timeout))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("nanabush: dial %s: %w", cfg.Address, err)
	}

	// TODO: Initialize generated client stub:
	// client := nanabushv1.NewTranslationServiceClient(conn)

	return &Client{
		conn:   conn,
		addr:   cfg.Address,
		secure: cfg.Secure,
		// client: client,
	}, nil
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// CheckTitleRequest represents a title-only pre-flight check.
type CheckTitleRequest struct {
	Title          string
	LanguageTag    string // Target language (e.g., "fr-CA")
	SourceLanguage string // Source language (e.g., "EN")
}

// CheckTitleResponse indicates if Nanabush is ready.
type CheckTitleResponse struct {
	Ready                bool
	Message              string
	EstimatedTimeSeconds int32
}

// CheckTitle performs a lightweight pre-flight check with title only.
// This validates that Nanabush is ready and can handle the request.
func (c *Client) CheckTitle(ctx context.Context, req CheckTitleRequest) (*CheckTitleResponse, error) {
	// TODO: Implement gRPC call once proto is compiled:
	// resp, err := c.client.CheckTitle(ctx, &nanabushv1.TitleCheckRequest{
	//     Title: req.Title,
	//     LanguageTag: req.LanguageTag,
	//     SourceLanguage: req.SourceLanguage,
	// })
	// if err != nil {
	//     return nil, fmt.Errorf("nanabush: CheckTitle: %w", err)
	// }
	// return &CheckTitleResponse{
	//     Ready: resp.Ready,
	//     Message: resp.Message,
	//     EstimatedTimeSeconds: resp.EstimatedTimeSeconds,
	// }, nil

	// Placeholder: return ready for now
	return &CheckTitleResponse{
		Ready:                true,
		Message:              "Ready (proto compilation pending)",
		EstimatedTimeSeconds: 30,
	}, nil
}

// DocumentContent represents document content and metadata.
type DocumentContent struct {
	Title    string
	Markdown string
	Slug     string
	Metadata map[string]string
}

// TranslateRequest contains the full translation request.
type TranslateRequest struct {
	JobID          string
	Namespace      string
	Primitive      string           // "title" or "doc-translate"
	Title          string           // For PRIMITIVE_TITLE
	Document       *DocumentContent // For PRIMITIVE_DOC_TRANSLATE
	TemplateHelper *DocumentContent // Optional template context
	SourceLanguage string
	TargetLanguage string
	SourceWikiURI  string
	PageID         string
	PageSlug       string
}

// TranslateResponse contains the translation result.
type TranslateResponse struct {
	JobID                string
	Success              bool
	TranslatedTitle      string
	TranslatedMarkdown   string
	ErrorMessage         string
	TokensUsed           int32
	InferenceTimeSeconds float64
	CompletedAt          time.Time
}

// Translate performs full document translation.
func (c *Client) Translate(ctx context.Context, req TranslateRequest) (*TranslateResponse, error) {
	// TODO: Implement gRPC call once proto is compiled:
	//
	// var sourceOneof *nanabushv1.TranslateRequest_Source
	// if req.Primitive == "title" {
	//     sourceOneof = &nanabushv1.TranslateRequest_Source{
	//         Source: &nanabushv1.TranslateRequest_Title{Title: req.Title},
	//     }
	// } else {
	//     sourceOneof = &nanabushv1.TranslateRequest_Source{
	//         Source: &nanabushv1.TranslateRequest_Doc{
	//             Doc: &nanabushv1.DocumentContent{
	//                 Title:    req.Document.Title,
	//                 Markdown: req.Document.Markdown,
	//                 Slug:     req.Document.Slug,
	//                 Metadata: req.Document.Metadata,
	//             },
	//         },
	//     }
	// }
	//
	// grpcReq := &nanabushv1.TranslateRequest{
	//     JobId:      req.JobID,
	//     Namespace:  req.Namespace,
	//     Primitive:  nanabushv1.PrimitiveType_PRIMITIVE_DOC_TRANSLATE,
	//     Source:     sourceOneof,
	//     SourceLanguage: req.SourceLanguage,
	//     TargetLanguage:  req.TargetLanguage,
	//     SourceWikiUri:  req.SourceWikiURI,
	//     PageId:         req.PageID,
	//     PageSlug:       req.PageSlug,
	//     RequestedAt:    timestamppb.Now(),
	// }
	//
	// if req.TemplateHelper != nil {
	//     grpcReq.TemplateHelper = &nanabushv1.DocumentContent{
	//         Title:    req.TemplateHelper.Title,
	//         Markdown: req.TemplateHelper.Markdown,
	//         Slug:     req.TemplateHelper.Slug,
	//         Metadata: req.TemplateHelper.Metadata,
	//     }
	// }
	//
	// resp, err := c.client.Translate(ctx, grpcReq)
	// if err != nil {
	//     return nil, fmt.Errorf("nanabush: Translate: %w", err)
	// }
	//
	// var completedAt time.Time
	// if resp.CompletedAt != nil {
	//     completedAt = resp.CompletedAt.AsTime()
	// }
	//
	// return &TranslateResponse{
	//     JobID:              resp.JobId,
	//     Success:            resp.Success,
	//     TranslatedTitle:    resp.TranslatedTitle,
	//     TranslatedMarkdown: resp.TranslatedMarkdown,
	//     ErrorMessage:       resp.ErrorMessage,
	//     TokensUsed:         resp.TokensUsed,
	//     InferenceTimeSeconds: resp.InferenceTimeSeconds,
	//     CompletedAt:        completedAt,
	// }, nil

	// Placeholder: return error indicating proto needs compilation
	return nil, fmt.Errorf("nanabush: gRPC proto not yet compiled - run 'make proto' or compile translation.proto to generate Go stubs")
}

// Helper function to convert DocumentContent to proto (for when proto is compiled)
func documentContentToProto(doc *DocumentContent) interface{} {
	// TODO: Return *nanabushv1.DocumentContent once proto is compiled
	return doc
}

// Helper function to convert proto timestamp (for when proto is compiled)
var _ = timestamppb.Now // Keep import for when we use it
