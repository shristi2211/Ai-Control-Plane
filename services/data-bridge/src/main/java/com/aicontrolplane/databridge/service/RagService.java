package com.aicontrolplane.databridge.service;

import dev.langchain4j.data.document.Document;
import dev.langchain4j.data.document.Metadata;
import dev.langchain4j.data.document.splitter.DocumentSplitters;
import dev.langchain4j.data.embedding.Embedding;
import dev.langchain4j.data.segment.TextSegment;
import dev.langchain4j.model.chat.ChatLanguageModel;
import dev.langchain4j.model.embedding.EmbeddingModel;
import dev.langchain4j.store.embedding.EmbeddingSearchRequest;
import dev.langchain4j.store.embedding.EmbeddingSearchResult;
import dev.langchain4j.store.embedding.EmbeddingStore;
import org.apache.pdfbox.Loader;
import org.apache.pdfbox.pdmodel.PDDocument;
import org.apache.pdfbox.text.PDFTextStripper;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.stereotype.Service;
import org.springframework.web.multipart.MultipartFile;

import java.io.IOException;
import java.util.List;
import java.util.stream.Collectors;

/**
 * RAG (Retrieval-Augmented Generation) Service.
 *
 * <p>Ingest: PDF → PDFBox → chunks → all-MiniLM-L6-v2 embeddings → pgvector
 * <p>Query: question → embed → similarity search → Gemini answers grounded in context
 */
@Service
public class RagService {

    private static final Logger log = LoggerFactory.getLogger(RagService.class);

    private final ChatLanguageModel chatModel;
    private final EmbeddingModel embeddingModel;
    private final EmbeddingStore<TextSegment> embeddingStore;

    /**
     * Grounding system prompt — forces AI to answer ONLY from provided context.
     * If the answer is not in context, it must say so (prevents hallucination).
     */
    private static final String RAG_SYSTEM_PROMPT = """
            You are a helpful document assistant. Answer the user's question ONLY based
            on the provided context chunks below. Follow these rules strictly:

            1. If the answer IS in the context, provide a clear and concise answer.
            2. If the answer is NOT in the context, say:
               "I don't have enough information in the uploaded documents to answer this question."
            3. Never make up information that is not in the context.
            4. If the context is partially relevant, say what you know and clearly state
               what information is missing.
            5. Cite which part of the context your answer comes from when possible.
            """;

    public RagService(ChatLanguageModel chatModel,
                      EmbeddingModel embeddingModel,
                      EmbeddingStore<TextSegment> embeddingStore) {
        this.chatModel = chatModel;
        this.embeddingModel = embeddingModel;
        this.embeddingStore = embeddingStore;
    }

    // ── Result Records ─────────────────────────────────────

    public record IngestResult(String fileName, int chunkCount, String error) {
        public static IngestResult success(String fileName, int chunkCount) {
            return new IngestResult(fileName, chunkCount, null);
        }
        public static IngestResult error(String error) {
            return new IngestResult(null, 0, error);
        }
    }

    public record AskResult(String answer, int chunksUsed, List<String> sourceChunks, String error) {
        public static AskResult success(String answer, int chunksUsed, List<String> sourceChunks) {
            return new AskResult(answer, chunksUsed, sourceChunks, null);
        }
        public static AskResult error(String error) {
            return new AskResult(null, 0, List.of(), error);
        }
    }

    // ── Ingest Pipeline ────────────────────────────────────

    /**
     * Ingests a PDF file: extract text → split into chunks → embed → store in pgvector.
     */
    public IngestResult ingest(MultipartFile file) {
        if (file == null || file.isEmpty()) {
            return IngestResult.error("No file provided or file is empty.");
        }

        String fileName = file.getOriginalFilename();
        if (fileName == null || !fileName.toLowerCase().endsWith(".pdf")) {
            return IngestResult.error("Only PDF files are supported.");
        }

        try {
            // Step 1: Extract text from PDF using PDFBox 3.0.3
            String pdfText = extractPdfText(file);
            if (pdfText.isBlank()) {
                return IngestResult.error("Could not extract any text from the PDF.");
            }

            log.info("[RAG INGEST] Extracted {} chars from '{}'", pdfText.length(), fileName);

            // Step 2: Create LangChain4j Document and split into chunks
            Document document = Document.from(pdfText, Metadata.from("source", fileName));
            var splitter = DocumentSplitters.recursive(500, 50);
            List<TextSegment> chunks = splitter.split(document);

            log.info("[RAG INGEST] Split into {} chunks", chunks.size());

            // Step 3: Embed all chunks and store in pgvector
            List<Embedding> embeddings = embeddingModel.embedAll(chunks).content();
            embeddingStore.addAll(embeddings, chunks);

            log.info("[RAG INGEST] Stored {} embeddings in pgvector", embeddings.size());

            return IngestResult.success(fileName, chunks.size());

        } catch (Exception e) {
            log.error("[RAG INGEST] Error processing '{}': {}", fileName, e.getMessage(), e);
            return IngestResult.error("Failed to ingest PDF: " + e.getMessage());
        }
    }

    // ── Query Pipeline ─────────────────────────────────────

    /**
     * Answers a question using RAG: embed question → similarity search → LLM with context.
     */
    public AskResult ask(String question) {
        if (question == null || question.isBlank()) {
            return AskResult.error("Question cannot be empty.");
        }

        try {
            // Step 1: Embed the question
            Embedding questionEmbedding = embeddingModel.embed(question).content();

            // Step 2: Find top-5 most similar chunks from pgvector
            EmbeddingSearchRequest searchRequest = EmbeddingSearchRequest.builder()
                    .queryEmbedding(questionEmbedding)
                    .maxResults(5)
                    .build();
            EmbeddingSearchResult<TextSegment> searchResult = embeddingStore.search(searchRequest);

            if (searchResult.matches().isEmpty()) {
                return AskResult.error("No documents have been ingested yet. Please upload a PDF first via POST /rag/ingest.");
            }

            // Step 3: Build context from matched chunks
            List<String> sourceChunks = searchResult.matches().stream()
                    .map(match -> match.embedded().text())
                    .collect(Collectors.toList());

            String context = sourceChunks.stream()
                    .map(chunk -> "---\n" + chunk + "\n---")
                    .collect(Collectors.joining("\n\n"));

            // Step 4: Ask Gemini with grounding prompt + context
            String fullPrompt = RAG_SYSTEM_PROMPT
                    + "\n\n=== CONTEXT FROM DOCUMENTS ===\n\n"
                    + context
                    + "\n\n=== USER QUESTION ===\n\n"
                    + question;

            String answer = chatModel.chat(fullPrompt);

            log.info("[RAG QUERY] Question: '{}', Chunks used: {}", question, searchResult.matches().size());

            return AskResult.success(answer, searchResult.matches().size(), sourceChunks);

        } catch (Exception e) {
            log.error("[RAG QUERY] Error: {}", e.getMessage(), e);
            return AskResult.error("Failed to process question: " + e.getMessage());
        }
    }

    // ── PDF Extraction ─────────────────────────────────────

    /**
     * Extracts text from a PDF using PDFBox 3.0.3 with SortByPosition=true
     * for better text ordering (especially tables) and whitespace cleanup.
     */
    private String extractPdfText(MultipartFile file) throws IOException {
        try (PDDocument pdf = Loader.loadPDF(file.getBytes())) {
            PDFTextStripper stripper = new PDFTextStripper();
            stripper.setSortByPosition(true);
            String text = stripper.getText(pdf);

            // Clean up: normalize whitespace, remove excessive blank lines
            text = text.replaceAll("\\r\\n", "\n")
                       .replaceAll("\\n{3,}", "\n\n")
                       .trim();

            return text;
        }
    }
}
