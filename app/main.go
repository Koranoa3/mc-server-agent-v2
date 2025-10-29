package main

import (
    "encoding/json"
    "fmt"
    "io/ioutil"
    "log"
    "os"
    "time"

    "github.com/joho/godotenv"
)

func main() {
    // Load .env if present
    _ = godotenv.Load()

    discordToken := os.Getenv("DISCORD_TOKEN")
    mcPath := os.Getenv("MC_SERVER_PATH")
    settingsPath := os.Getenv("SETTINGS_PATH")
    if settingsPath == "" {
        settingsPath = "settings.json"
    }

    fmt.Printf("DISCORD_TOKEN present: %v\n", discordToken != "")
    fmt.Printf("MC_SERVER_PATH: %s\n", mcPath)
    fmt.Printf("Using settings file: %s\n", settingsPath)

    // Read settings.json
    data, err := ioutil.ReadFile(settingsPath)
    if err != nil {
        log.Fatalf("failed to read settings file %s: %v", settingsPath, err)
    }

    var settings map[string]interface{}
    if err := json.Unmarshal(data, &settings); err != nil {
        log.Fatalf("failed to parse settings.json: %v", err)
    }

    // Print a short summary: registered container keys if present
    if rc, ok := settings["registered_containers"].(map[string]interface{}); ok {
        fmt.Printf("Registered containers:\n")
        for k := range rc {
            fmt.Printf(" - %s\n", k)
        }
    } else {
        fmt.Printf("no registered_containers found in settings.json\n")
    }

    // Demonstrate write access: set last_started timestamp
    settings["last_started"] = time.Now().Format(time.RFC3339)

    out, err := json.MarshalIndent(settings, "", "  ")
    if err != nil {
        log.Fatalf("failed to marshal settings: %v", err)
    }

    if err := ioutil.WriteFile(settingsPath, out, 0644); err != nil {
        log.Fatalf("failed to write settings file %s: %v", settingsPath, err)
    }

    fmt.Printf("Updated %s with last_started timestamp\n", settingsPath)
}
