package com.aicontrolplane.monolith.controller;

import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.Map;

/**
 * Admin API Controller — Users, Payments, UI API endpoints.
 * Kong routes /v1/admin/* here after stripping the prefix.
 */
@RestController
public class AdminController {

    public AdminController() {
        super();
    }

    @GetMapping("/health")
    public ResponseEntity<Map<String, Object>> health() {
        return ResponseEntity.ok(Map.of(
                "service", "monolith-core",
                "status", "healthy",
                "timestamp", Instant.now().toString()
        ));
    }

    @RequestMapping("/**")
    public ResponseEntity<Map<String, Object>> catchAll(@RequestHeader(value = "X-Internal-Key", required = false) String internalKey) {
        return ResponseEntity.ok(Map.of(
                "service", "monolith-core",
                "status", "ok",
                "message", "Monolith Core — Users, Payments, UI API",
                "timestamp", Instant.now().toString()
        ));
    }
}
