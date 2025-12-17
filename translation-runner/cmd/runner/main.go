package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/nanabush"
	"github.com/dasmlab/glooscap-operator/pkg/outline"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func main() {
	var translationJobRef string
	var translationServiceAddr string
	flag.StringVar(&translationJobRef, "translation-job", "", "TranslationJob reference in format namespace/name")
	flag.StringVar(&translationServiceAddr, "translation-service-addr", "", "Translation service gRPC address (or use TRANSLATION_SERVICE_ADDR env)")
	flag.Parse()

	if translationJobRef == "" {
		fmt.Fprintf(os.Stderr, "error: --translation-job is required\n")
		os.Exit(1)
	}

	// Step 1: Job is scheduled, runner is pulled, data is passed
	fmt.Println("========================================")
	fmt.Println("Translation Runner - Starting")
	fmt.Println("========================================")
	fmt.Printf("Step 1: Job scheduled, data received\n")
	fmt.Printf("  TranslationJob: %s\n", translationJobRef)

	// Get translation service address from env if not provided
	if translationServiceAddr == "" {
		translationServiceAddr = os.Getenv("TRANSLATION_SERVICE_ADDR")
	}
	if translationServiceAddr == "" {
		translationServiceAddr = "iskoces-service.iskoces.svc.cluster.local:50051" // Default
	}
	fmt.Printf("  Translation Service: %s\n", translationServiceAddr)

	// Parse namespace/name
	var namespace, name string
	parts := splitNamespaceName(translationJobRef)
	if len(parts) != 2 {
		fmt.Fprintf(os.Stderr, "error: invalid translation-job format, expected namespace/name, got: %s\n", translationJobRef)
		os.Exit(1)
	}
	namespace, name = parts[0], parts[1]

	// Create Kubernetes client
	cfg, err := config.GetConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to get kubeconfig: %v\n", err)
		os.Exit(1)
	}

	// Add our API types to the scheme
	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to add core scheme: %v\n", err)
		os.Exit(1)
	}
	if err := wikiv1alpha1.AddToScheme(s); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to add wiki scheme: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create k8s client: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Get TranslationJob CR
	var job wikiv1alpha1.TranslationJob
	if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &job); err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to get TranslationJob %s/%s: %v\n", namespace, name, err)
		os.Exit(1)
	}

	fmt.Printf("  Job Name: %s\n", job.Name)
	fmt.Printf("  Source Target: %s, PageID: %s\n", job.Spec.Source.TargetRef, job.Spec.Source.PageID)
	if job.Spec.Destination != nil {
		fmt.Printf("  Destination Target: %s, Language: %s\n", job.Spec.Destination.TargetRef, job.Spec.Destination.LanguageTag)
	}
	
	// Check if this is a publish job
	isPublishJob := job.Spec.Parameters["publish"] == "true"
	if isPublishJob {
		fmt.Printf("  This is a PUBLISH job (publishing draft page)\n")
		fmt.Printf("  Original Job: %s\n", job.Spec.Parameters["originalJob"])
		fmt.Printf("  Page ID to publish: %s\n", job.Spec.Parameters["pageId"])
	}

	// Check if this is a diagnostic job
	isDiagnostic := job.Labels["glooscap.dasmlab.org/diagnostic"] == "true" ||
		job.Spec.Parameters["diagnostic"] == "true"
	prefix := "AUTOTRANSLATED"
	if isDiagnostic {
		prefix = "AUTODIAG"
		fmt.Printf("  Diagnostic job detected - will use %s prefix\n", prefix)
	}

	// Update job status to Running
	now := metav1.Now()
	job.Status.State = wikiv1alpha1.TranslationJobStateRunning
	job.Status.Message = "Translation runner processing"
	if job.Status.StartedAt == nil {
		job.Status.StartedAt = &now
	}
	if err := k8sClient.Status().Update(ctx, &job); err != nil {
		fmt.Printf("warning: failed to update job status: %v\n", err)
	}

	// Step 2: Source page is pulled down and handled locally
	fmt.Println("\nStep 2: Fetching source page content")
	fmt.Println("----------------------------------------")

	// Get source WikiTarget (skip for diagnostic jobs with embedded content)
	var sourceTarget wikiv1alpha1.WikiTarget
	testContent := job.Spec.Parameters["testContent"]
	fmt.Printf("DEBUG: isDiagnostic=%v, testContent present=%v, testContent length=%d\n", isDiagnostic, testContent != "", len(testContent))
	if isDiagnostic && testContent != "" {
		// Diagnostic job with embedded content - create dummy WikiTarget
		fmt.Printf("Diagnostic job with embedded content - using dummy WikiTarget\n")
		sourceTarget = wikiv1alpha1.WikiTarget{
			ObjectMeta: metav1.ObjectMeta{
				Name:      job.Spec.Source.TargetRef,
				Namespace: namespace,
			},
			Spec: wikiv1alpha1.WikiTargetSpec{
				URI: "diagnostic://test",
			},
		}
	} else {
		// Regular job - need WikiTarget
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: job.Spec.Source.TargetRef}, &sourceTarget); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to get source WikiTarget %s: %v\n", job.Spec.Source.TargetRef, err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to get source target: %v", err))
			os.Exit(1)
		}
	}

	// Create Outline client helper function
	createOutlineClient := func(target *wikiv1alpha1.WikiTarget) (*outline.Client, error) {
		if target.Spec.ServiceAccountSecretRef.Name == "" {
			return nil, fmt.Errorf("service account secret ref is empty")
		}

		var secret corev1.Secret
		key := types.NamespacedName{
			Namespace: target.Namespace,
			Name:      target.Spec.ServiceAccountSecretRef.Name,
		}
		if err := k8sClient.Get(ctx, key, &secret); err != nil {
			return nil, fmt.Errorf("get secret %s: %w", key, err)
		}

		keyName := target.Spec.ServiceAccountSecretRef.Key
		if keyName == "" {
			keyName = "token"
		}

		tokenBytes, ok := secret.Data[keyName]
		if !ok {
			return nil, fmt.Errorf("key %q not found in secret %s", keyName, key)
		}

		token := strings.TrimSpace(string(tokenBytes))
		return outline.NewClient(outline.Config{
			BaseURL: target.Spec.URI,
			Token:   token,
		})
	}

	// Handle publish job (publish draft page)
	if isPublishJob {
		fmt.Println("\nPublish Job: Publishing draft page")
		fmt.Println("----------------------------------------")
		
		pageID := job.Spec.Parameters["pageId"]
		if pageID == "" {
			pageID = job.Spec.Source.PageID // Fallback to Source.PageID
		}
		
		if pageID == "" {
			fmt.Fprintf(os.Stderr, "error: page ID not found in publish job parameters\n")
			updateJobStatusFailed(ctx, k8sClient, &job, "Page ID not found in publish job parameters")
			os.Exit(1)
		}
		
		// Get destination WikiTarget (same as source for publish jobs, skip for diagnostic)
		var destTarget wikiv1alpha1.WikiTarget
		destTargetRef := job.Spec.Source.TargetRef
		if job.Spec.Parameters["targetRef"] != "" {
			destTargetRef = job.Spec.Parameters["targetRef"]
		}
		if isDiagnostic && testContent != "" {
			// Use dummy target for diagnostic jobs
			destTarget = wikiv1alpha1.WikiTarget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      destTargetRef,
					Namespace: namespace,
				},
				Spec: wikiv1alpha1.WikiTargetSpec{
					URI:  "diagnostic://test",
					Mode: wikiv1alpha1.WikiTargetModeReadWrite,
				},
			}
		} else {
			if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: destTargetRef}, &destTarget); err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to get destination WikiTarget %s: %v\n", destTargetRef, err)
				updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to get destination target: %v", err))
				os.Exit(1)
			}
		}
		
		// Create destination Outline client
		destClient, err := createOutlineClient(&destTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to create destination Outline client: %v\n", err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to create destination client: %v", err))
			os.Exit(1)
		}
		
		// Publish the draft page
		fmt.Printf("Publishing page ID: %s\n", pageID)
		publishResp, err := destClient.PublishPage(ctx, outline.PublishPageRequest{ID: pageID})
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to publish page: %v\n", err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to publish page: %v", err))
			os.Exit(1)
		}
		
		fmt.Printf("✓ Page published successfully\n")
		fmt.Printf("  Page ID: %s\n", publishResp.Data.ID)
		fmt.Printf("  Title: %s\n", publishResp.Data.Title)
		fmt.Printf("  Slug: %s\n", publishResp.Data.Slug)
		
		// Build page URL
		pageURL := ""
		if destTarget.Spec.URI != "" {
			pageURL = fmt.Sprintf("%s/doc/%s", strings.TrimSuffix(destTarget.Spec.URI, "/"), publishResp.Data.Slug)
			fmt.Printf("  URL: %s\n", pageURL)
		}
		
		// Update job status to Completed
		now := metav1.Now()
		job.Status.State = wikiv1alpha1.TranslationJobStateCompleted
		job.Status.FinishedAt = &now
		job.Status.Message = fmt.Sprintf("Page published successfully (page: %s)", publishResp.Data.Slug)
		
		// Store published page info in annotations
		if job.Annotations == nil {
			job.Annotations = make(map[string]string)
		}
		job.Annotations["glooscap.dasmlab.org/published-page-id"] = publishResp.Data.ID
		job.Annotations["glooscap.dasmlab.org/published-page-slug"] = publishResp.Data.Slug
		job.Annotations["glooscap.dasmlab.org/published-page-url"] = pageURL
		job.Annotations["glooscap.dasmlab.org/is-draft"] = "false"
		
		if err := k8sClient.Update(ctx, &job); err != nil {
			fmt.Printf("warning: failed to update job annotations: %v\n", err)
		}
		
		if err := k8sClient.Status().Update(ctx, &job); err != nil {
			fmt.Printf("warning: failed to update job status to completed: %v\n", err)
		} else {
			fmt.Printf("✓ Job status updated to Completed\n")
		}
		
		os.Exit(0)
	}
	
	// Create source Outline client (for regular translation jobs, skip for diagnostic with embedded content)
	var sourceClient *outline.Client
	if !isDiagnostic || job.Spec.Parameters["testContent"] == "" {
		// Only create client if we have a real WikiTarget
		var err error
		sourceClient, err = createOutlineClient(&sourceTarget)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to create source Outline client: %v\n", err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to create source client: %v", err))
			os.Exit(1)
		}
	}

	// Fetch source page content (or use embedded test content for diagnostics)
	var pageContent *outline.PageContent
	var sourcePageTitle string
	var sourcePageSlug string
	var sourceCollectionID string
	
	if isDiagnostic && job.Spec.Parameters["testContent"] != "" {
		// Use embedded test content for diagnostic jobs
		fmt.Printf("Using embedded test content for diagnostic job\n")
		testContent := job.Spec.Parameters["testContent"]
		pageTitle := job.Spec.Parameters["pageTitle"]
		if pageTitle == "" {
			pageTitle = "Diagnostic Test"
		}
		
		pageContent = &outline.PageContent{
			ID:       job.Spec.Source.PageID,
			Title:    pageTitle,
			Markdown: testContent,
			Slug:     strings.ToLower(strings.ReplaceAll(pageTitle, " ", "-")),
		}
		sourcePageTitle = pageTitle
		sourcePageSlug = pageContent.Slug
		sourceCollectionID = ""
	} else {
		// Fetch from source wiki
		fmt.Printf("Fetching page content for pageID: %s\n", job.Spec.Source.PageID)
		var err error
		pageContent, err = sourceClient.GetPageContent(ctx, job.Spec.Source.PageID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to fetch page content: %v\n", err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to fetch page content: %v", err))
			os.Exit(1)
		}
		
		// Get page metadata (title, slug, collection)
		sourcePages, err := sourceClient.ListPages(ctx)
		if err != nil {
			fmt.Printf("warning: failed to list pages for metadata: %v\n", err)
		}
		for _, p := range sourcePages {
			if p.ID == job.Spec.Source.PageID {
				sourcePageTitle = p.Title
				sourcePageSlug = p.Slug
				sourceCollectionID = p.Collection
				break
			}
		}
		if sourcePageTitle == "" && pageContent.Title != "" {
			sourcePageTitle = pageContent.Title
		}
		if sourcePageSlug == "" && pageContent.Slug != "" {
			sourcePageSlug = pageContent.Slug
		}
	}

	fmt.Printf("✓ Source page fetched successfully\n")
	fmt.Printf("  Title: %s\n", sourcePageTitle)
	fmt.Printf("  Slug: %s\n", sourcePageSlug)
	fmt.Printf("  Collection: %s\n", sourceCollectionID)
	fmt.Printf("  Content length: %d characters\n", len(pageContent.Markdown))

	// Step 3: Translation service is called and response is retrieved
	fmt.Println("\nStep 3: Calling translation service")
	fmt.Println("----------------------------------------")

	// Determine target language
	targetLang := "fr-CA" // Default
	if job.Spec.Destination != nil && job.Spec.Destination.LanguageTag != "" {
		targetLang = job.Spec.Destination.LanguageTag
	}

	// Determine source language (default to en)
	sourceLang := "en"

	// Create translation service client (portable gRPC client)
	fmt.Printf("Connecting to translation service: %s\n", translationServiceAddr)
	nanabushClient, err := nanabush.NewClient(nanabush.Config{
		Address:       translationServiceAddr,
		Secure:        false, // TODO: make configurable
		ClientName:    "glooscap-translation-runner",
		ClientVersion: "1.0.0",
		Namespace:     namespace,
		Timeout:       30 * time.Second,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create translation service client: %v\n", err)
		updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to connect to translation service: %v", err))
		os.Exit(1)
	}
	// Note: nanabush client manages its own connection lifecycle

	fmt.Printf("Translating page (source: %s -> target: %s)...\n", sourceLang, targetLang)
	fmt.Printf("Source content preview (first 200 chars):\n%s\n", truncateString(pageContent.Markdown, 200))
	
	translateReq := nanabush.TranslateRequest{
		JobID:     job.Name,
		Namespace: namespace,
		Primitive: "doc-translate",
		Document: &nanabush.DocumentContent{
			Title:    sourcePageTitle,
			Markdown: pageContent.Markdown,
			Slug:     sourcePageSlug,
		},
		SourceLanguage: sourceLang,
		TargetLanguage: targetLang,
		SourceWikiURI:  sourceTarget.Spec.URI,
		PageID:         job.Spec.Source.PageID,
		PageSlug:       sourcePageSlug,
	}

	fmt.Printf("Calling translation service with:\n")
	fmt.Printf("  JobID: %s\n", translateReq.JobID)
	fmt.Printf("  Primitive: %s\n", translateReq.Primitive)
	fmt.Printf("  Source Language: %s\n", translateReq.SourceLanguage)
	fmt.Printf("  Target Language: %s\n", translateReq.TargetLanguage)
	fmt.Printf("  Document Title: %s\n", translateReq.Document.Title)
	fmt.Printf("  Document Length: %d chars\n", len(translateReq.Document.Markdown))

	translateResp, err := nanabushClient.Translate(ctx, translateReq)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: translation failed: %v\n", err)
		updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Translation failed: %v", err))
		os.Exit(1)
	}

	fmt.Printf("Translation service response received:\n")
	fmt.Printf("  Success: %v\n", translateResp.Success)
	fmt.Printf("  JobID: %s\n", translateResp.JobID)
	fmt.Printf("  Tokens used: %d\n", translateResp.TokensUsed)
	fmt.Printf("  Inference time: %.2fs\n", translateResp.InferenceTimeSeconds)
	if translateResp.ErrorMessage != "" {
		fmt.Printf("  Error Message: %s\n", translateResp.ErrorMessage)
	}

	if !translateResp.Success {
		fmt.Fprintf(os.Stderr, "error: translation service returned error: %s\n", translateResp.ErrorMessage)
		updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Translation failed: %s", translateResp.ErrorMessage))
		os.Exit(1)
	}

	fmt.Printf("✓ Translation completed successfully\n")
	fmt.Printf("  Translated Title: %s\n", translateResp.TranslatedTitle)
	fmt.Printf("  Translated content length: %d characters\n", len(translateResp.TranslatedMarkdown))
	fmt.Printf("  Translated content preview (first 500 chars):\n%s\n", truncateString(translateResp.TranslatedMarkdown, 500))

	// Step 4: Create target destination page with PREFIX (skip for diagnostic jobs)
	if isDiagnostic {
		// Diagnostic jobs just test the translation service - don't publish
		fmt.Println("\nStep 4: Diagnostic job - skipping wiki publish (translation service test complete)")
		fmt.Println("----------------------------------------")
		fmt.Printf("✓ Translation service test successful!\n")
		fmt.Printf("  Source text length: %d characters\n", len(pageContent.Markdown))
		fmt.Printf("  Translated text length: %d characters\n", len(translateResp.TranslatedMarkdown))
		fmt.Printf("  Source language: %s\n", sourceLang)
		fmt.Printf("  Target language: %s\n", targetLang)
		
		// Update job status to Completed (without publishing)
		now := metav1.Now()
		job.Status.State = wikiv1alpha1.TranslationJobStateCompleted
		job.Status.FinishedAt = &now
		job.Status.Message = "Translation service test completed successfully (no wiki publish)"
		
		if err := k8sClient.Status().Update(ctx, &job); err != nil {
			fmt.Printf("warning: failed to update job status: %v\n", err)
		} else {
			fmt.Printf("✓ Job status updated to Completed\n")
		}
		
		os.Exit(0)
	}

	// Step 4: Create target destination page with PREFIX
	fmt.Println("\nStep 4: Creating destination page with prefix")
	fmt.Println("----------------------------------------")

	// Get destination WikiTarget (skip for diagnostic jobs - they don't publish)
	var destTarget wikiv1alpha1.WikiTarget
	if isDiagnostic {
		// Diagnostic jobs don't publish to wiki - just test translation service
		fmt.Printf("Diagnostic job - skipping destination WikiTarget fetch (results will be logged only)\n")
	} else {
		// Regular job - need destination WikiTarget
		destTargetRef := job.Spec.Source.TargetRef
		if job.Spec.Destination != nil && job.Spec.Destination.TargetRef != "" {
			destTargetRef = job.Spec.Destination.TargetRef
		}
		
		if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: destTargetRef}, &destTarget); err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to get destination WikiTarget %s: %v\n", destTargetRef, err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to get destination target: %v", err))
			os.Exit(1)
		}
	}

	// Create destination Outline client
	destClient, err := createOutlineClient(&destTarget)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to create destination Outline client: %v\n", err)
		updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to create destination client: %v", err))
		os.Exit(1)
	}

	// Build page title with prefix
	baseTitle := sourcePageTitle
	if baseTitle == "" {
		baseTitle = "Untitled Page"
	}

	var translatedTitle string
	var collectionID string
	var finalContent string
	var createResp *outline.CreatePageResponse // Declare here for use in both branches

	if isDiagnostic {
		// Diagnostic jobs: AUTODIAG prefix, GLOOSCAP-DIAG collection
		translatedTitle = fmt.Sprintf("%s--> %s", prefix, baseTitle)
		
		// Get or create GLOOSCAP-DIAG collection
		fmt.Printf("Ensuring GLOOSCAP-DIAG collection exists...\n")
		diagCollectionID, err := destClient.GetOrCreateCollection(ctx, "GLOOSCAP-DIAG")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to get/create GLOOSCAP-DIAG collection: %v\n", err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to get/create collection: %v", err))
			os.Exit(1)
		}
		collectionID = diagCollectionID
		fmt.Printf("Using collection ID: %s\n", collectionID)
		
		// Generate UUID marker for this run
		uuid := fmt.Sprintf("%d-%x", time.Now().Unix(), time.Now().UnixNano()%10000)
		marker := fmt.Sprintf("\n\n---\n*Diagnostic job: %s/%s, UUID: %s, Generated: %s*\n",
			namespace, name, uuid, time.Now().Format(time.RFC3339))
		
		// Base content without marker (for comparison)
		baseContent := translateResp.TranslatedMarkdown
		finalContent = baseContent + marker
		
		// Check if page with same title exists (for diagnostic jobs, always update existing)
		// Check both drafts and published pages
		fmt.Printf("Checking for existing page with title: %s (including drafts)\n", translatedTitle)
		destPages, err := destClient.ListPages(ctx)
		var existingPageID string
		if err == nil {
			for _, dp := range destPages {
				// Match by exact title (drafts and published pages both have titles)
				if dp.Title == translatedTitle {
					existingPageID = dp.ID
					fmt.Printf("Found existing page with ID: %s (isDraft: %v)\n", existingPageID, dp.IsDraft)
					break
				}
			}
		} else {
			fmt.Printf("warning: failed to list pages to check for existing: %v\n", err)
		}
		
		if existingPageID != "" {
			// For diagnostic jobs, always update existing page (add new UUID marker)
			fmt.Printf("Updating existing diagnostic page with new UUID marker...\n")
			updateReq := outline.UpdatePageRequest{
				ID:   existingPageID,
				Text: finalContent, // This includes the new UUID marker
			}
			updateResp, err := destClient.UpdatePage(ctx, updateReq)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: failed to update page: %v\n", err)
				updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to update page: %v", err))
				os.Exit(1)
			}
			
			// Keep the page as draft (don't publish)
			fmt.Printf("✓ Page updated successfully (kept as draft)\n")
			
			createResp = &outline.CreatePageResponse{
				Data: struct {
					ID    string `json:"id"`
					Title string `json:"title"`
					Slug  string `json:"urlId"`
				}{
					ID:    updateResp.Data.ID,
					Title: updateResp.Data.Title,
					Slug:  updateResp.Data.Slug,
				},
			}
		}
		
		// If no existing page, create new
		if existingPageID == "" {
			// Will create new page below
		}
	} else {
		// Regular jobs: AUTOTRANSLATED prefix, same collection as source
		translatedTitle = fmt.Sprintf("%s--> %s", prefix, baseTitle)
		collectionID = sourceCollectionID
		finalContent = translateResp.TranslatedMarkdown

		// Check for existing pages with same title (for regular jobs, don't overwrite)
		destPages, err := destClient.ListPages(ctx)
		if err == nil {
			uniqueTitle := translatedTitle
			counter := 1
			for {
				titleExists := false
				for _, dp := range destPages {
					if dp.Title == uniqueTitle {
						titleExists = true
						break
					}
				}
				if !titleExists {
					break
				}
				uniqueTitle = fmt.Sprintf("%s--> %s (%d)", prefix, baseTitle, counter)
				counter++
				if counter > 100 {
					break
				}
			}
			translatedTitle = uniqueTitle
		}
	}

	// Create the translated page (if not already updated above)
	if createResp == nil {
		fmt.Printf("Creating page with:\n")
		fmt.Printf("  Title: %s\n", translatedTitle)
		fmt.Printf("  Collection ID: %s (empty = top level)\n", collectionID)
		fmt.Printf("  Content length: %d characters\n", len(finalContent))
		fmt.Printf("  Content preview (first 300 chars):\n%s\n", truncateString(finalContent, 300))
		
		createReq := outline.CreatePageRequest{
			Title:        translatedTitle,
			Text:         finalContent,
			CollectionID: collectionID,
		}

		var err error
		createResp, err = destClient.CreatePage(ctx, createReq)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to create translated page: %v\n", err)
			updateJobStatusFailed(ctx, k8sClient, &job, fmt.Sprintf("Failed to create page: %v", err))
			os.Exit(1)
		}

		fmt.Printf("✓ Destination page created successfully\n")
		fmt.Printf("  Page ID: %s\n", createResp.Data.ID)
		fmt.Printf("  Title: %s\n", createResp.Data.Title)
		fmt.Printf("  Slug: %s\n", createResp.Data.Slug)
	}

	// Build page URL
	pageURL := ""
	if destTarget.Spec.URI != "" {
		pageURL = fmt.Sprintf("%s/doc/%s", strings.TrimSuffix(destTarget.Spec.URI, "/"), createResp.Data.Slug)
		fmt.Printf("  URL: %s\n", pageURL)
	}

	// Step 5: Exit and log results
	fmt.Println("\nStep 5: Updating job status and exiting")
	fmt.Println("----------------------------------------")

	// Update job status to AwaitingApproval (page created as draft, waiting for user approval)
	job.Status.State = wikiv1alpha1.TranslationJobStateAwaitingApproval
	job.Status.Message = fmt.Sprintf("Translation completed and created as draft (page: %s). Awaiting approval to publish.", createResp.Data.Slug)
	
	// Store published page info in annotations for UI to access
	if job.Annotations == nil {
		job.Annotations = make(map[string]string)
	}
	job.Annotations["glooscap.dasmlab.org/published-page-id"] = createResp.Data.ID
	job.Annotations["glooscap.dasmlab.org/published-page-slug"] = createResp.Data.Slug
	job.Annotations["glooscap.dasmlab.org/published-page-url"] = pageURL
	job.Annotations["glooscap.dasmlab.org/is-draft"] = "true"
	
	if err := k8sClient.Update(ctx, &job); err != nil {
		fmt.Printf("warning: failed to update job annotations: %v\n", err)
	}
	
	if err := k8sClient.Status().Update(ctx, &job); err != nil {
		fmt.Printf("warning: failed to update job status to completed: %v\n", err)
	} else {
		fmt.Printf("✓ Job status updated to Completed (draft)\n")
	}

	fmt.Println("\n========================================")
	fmt.Println("Translation Runner - Completed Successfully")
	fmt.Println("========================================")
	fmt.Printf("Summary:\n")
	fmt.Printf("  Source: %s (page: %s)\n", sourceTarget.Spec.URI, sourcePageTitle)
	fmt.Printf("  Destination: %s\n", destTarget.Spec.URI)
	fmt.Printf("  Translated Page: %s\n", pageURL)
	fmt.Printf("  Translation Time: %.2fs\n", translateResp.InferenceTimeSeconds)
	fmt.Printf("  Tokens Used: %d\n", translateResp.TokensUsed)
	fmt.Println("========================================")

	os.Exit(0)
}

func updateJobStatusFailed(ctx context.Context, k8sClient client.Client, job *wikiv1alpha1.TranslationJob, message string) {
	now := metav1.Now()
	job.Status.State = wikiv1alpha1.TranslationJobStateFailed
	job.Status.FinishedAt = &now
	job.Status.Message = message
	_ = k8sClient.Status().Update(ctx, job)
	fmt.Printf("\n✗ Job failed: %s\n", message)
}

func splitNamespaceName(ref string) []string {
	parts := make([]string, 0, 2)
	lastIdx := 0
	for i, r := range ref {
		if r == '/' {
			if i > lastIdx {
				parts = append(parts, ref[lastIdx:i])
			}
			lastIdx = i + 1
		}
	}
	if lastIdx < len(ref) {
		parts = append(parts, ref[lastIdx:])
	}
	return parts
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// removeUUIDMarker removes the UUID marker from the end of content for comparison
func removeUUIDMarker(content string) string {
	// Look for the marker pattern: ---\n*Diagnostic job: ...*
	markerStart := strings.LastIndex(content, "\n\n---\n*Diagnostic job:")
	if markerStart >= 0 {
		return content[:markerStart]
	}
	return content
}
