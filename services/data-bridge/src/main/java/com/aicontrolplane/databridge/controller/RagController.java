package com.aicontrolplane.databridge.controller;

import com.aicontrolplane.databridge.service.RagService;
import com.aicontrolplane.databridge.service.RagService.AskResult;
import com.aicontrolplane.databridge.service.RagService.IngestResult;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.multipart.MultipartFile;

import java.time.Instant;
import java.util.Map;

/**
 * REST controller for RAG (Retrieval-Augmented Generation) endpoints.
 * Handles PDF ingestion and document-grounded Q&A.
 */
@RestController
@RequestMapping("/rag")
public class RagController {

    private final RagService ragService;

    public RagController(RagService ragService) {
        this.ragService = ragService;
    }

    /**
     * POST /rag/ingest — upload a PDF file to be chunked, embedded, and stored.
     */
    @PostMapping("/ingest")
    public ResponseEntity<Map<String, Object>> ingest(@RequestParam("file") MultipartFile file) {
        IngestResult result = ragService.ingest(file);

        if (result.error() != null) {
            return ResponseEntity.badRequest().body(Map.of(
                    "error", "Ingest Failed",
                    "code", 400,
                    "message", result.error()
            ));
        }

        return ResponseEntity.ok(Map.of(
                "service", "data-bridge",
                "status", "ingested",
                "file_name", result.fileName(),
                "chunks_created", result.chunkCount(),
                "timestamp", Instant.now().toString()
        ));
    }

    /**
     * POST /rag/ask — ask a question against ingested documents.
     *
     * Request:  {"question": "What does the document say about data privacy?"}
     * Response: {"answer": "...", "chunks_used": 5, "source_chunks": [...]}
     */
    @PostMapping("/ask")
    public ResponseEntity<Map<String, Object>> ask(@RequestBody Map<String, String> body) {
        String question = body.get("question");

        if (question == null || question.isBlank()) {
            return ResponseEntity.badRequest().body(Map.of(
                    "error", "Bad Request",
                    "code", 400,
                    "message", "The 'question' field is required."
            ));
        }

        AskResult result = ragService.ask(question);

        if (result.error() != null) {
            return ResponseEntity.badRequest().body(Map.of(
                    "error", "Query Failed",
                    "code", 400,
                    "message", result.error()
            ));
        }

        return ResponseEntity.ok(Map.of(
                "service", "data-bridge",
                "answer", result.answer(),
                "chunks_used", result.chunksUsed(),
                "source_chunks", result.sourceChunks(),
                "timestamp", Instant.now().toString()
        ));
    }
}
