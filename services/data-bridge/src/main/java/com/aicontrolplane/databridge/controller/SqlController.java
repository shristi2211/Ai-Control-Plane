package com.aicontrolplane.databridge.controller;

import com.aicontrolplane.databridge.service.TextToSqlService;
import com.aicontrolplane.databridge.service.TextToSqlService.SqlResult;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.Map;

/**
 * REST controller for Text-to-SQL endpoint.
 * Accepts natural language questions and returns SQL results.
 */
@RestController
@RequestMapping("/sql")
public class SqlController {

    private final TextToSqlService textToSqlService;

    public SqlController(TextToSqlService textToSqlService) {
        this.textToSqlService = textToSqlService;
    }

    /**
     * POST /sql/ask — convert natural language to SQL and execute.
     *
     * Request:  {"question": "Total sales in North region?"}
     * Response: {"sql": "SELECT ...", "results": [...], "row_count": N}
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

        SqlResult result = textToSqlService.ask(question);

        if (result.error() != null) {
            return ResponseEntity.badRequest().body(Map.of(
                    "error", "Query Failed",
                    "code", 400,
                    "message", result.error()
            ));
        }

        return ResponseEntity.ok(Map.of(
                "service", "data-bridge",
                "sql", result.sql(),
                "results", result.results(),
                "row_count", result.rowCount(),
                "timestamp", Instant.now().toString()
        ));
    }
}
