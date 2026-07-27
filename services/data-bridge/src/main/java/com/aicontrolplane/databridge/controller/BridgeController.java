package com.aicontrolplane.databridge.controller;

import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;

import java.time.Instant;
import java.util.Map;

/**
 * Data Bridge Controller — Legacy SQL / Oracle connector endpoints.
 * Called internally by agent-brain via Docker DNS (http://data-bridge:8083).
 */
@RestController
public class BridgeController {

    @GetMapping("/health")
    public ResponseEntity<Map<String, Object>> health() {
        return ResponseEntity.ok(Map.of(
                "service", "data-bridge",
                "status", "healthy",
                "timestamp", Instant.now().toString()
        ));
    }

    @GetMapping("/query")
    public ResponseEntity<Map<String, Object>> query(@RequestParam(defaultValue = "default") String source) {
        String legacyUrl = switch (source.toLowerCase()) {
            case "oracle" -> "jdbc:oracle:thin:@legacy-db:1521:orcl";
            case "sqlserver" -> "jdbc:sqlserver://legacy-db:1433;databaseName=core";
            default -> "jdbc:postgresql://legacy-db:5432/defaultdb";
        };

        return ResponseEntity.ok(Map.of(
                "service", "data-bridge",
                "status", "ok",
                "message", "Data Bridge connector — source: " + source,
                "legacy_route", legacyUrl,
                "timestamp", Instant.now().toString()
        ));
    }

    @RequestMapping("/**")
    public ResponseEntity<Map<String, Object>> catchAll() {
        return ResponseEntity.ok(Map.of(
                "service", "data-bridge",
                "status", "ok",
                "message", "Data Bridge — Legacy SQL/Oracle Connectors",
                "timestamp", Instant.now().toString()
        ));
    }
}
