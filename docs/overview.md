## **Bus Delay Tracker - High Level Architecture**

### **Stack**
- **Go** - everything in Go, keep it simple
- **Redpanda** - single broker is fine
- **Parquet** - store in local filesystem (medallion layers)
- **Prometheus + Grafana** - observability

### **Data Flow**

```
API Poller → Redpanda → Processor → Parquet (Bronze/Silver/Gold)
                ↓
          Prometheus metrics → Grafana dashboard
```

### **3 Simple Services**

**1. Ingester Service** (`cmd/ingester`)
- Poll `/autobusi` every 30 seconds
- Push raw JSON to Redpanda topic `bus-positions-raw`
- Expose metrics: API latency, poll count, errors
- That's it

**2. Processor Service** (`cmd/processor`)
- Consume from `bus-positions-raw`
- **Bronze**: Write raw events to `data/bronze/YYYY-MM-DD/positions.parquet`
- **Silver**: Match bus position to nearest station, calculate if early/late based on schedule
- Write to `data/silver/YYYY-MM-DD/delays.parquet` 
- Emit to Redpanda topic `bus-delays`
- Metrics: processing lag, delay distribution

**3. Aggregator Service** (`cmd/aggregator`)
- Consume from `bus-delays`
- **Gold**: Calculate hourly stats per route (avg delay, p95, p99)
- Write to `data/gold/YYYY-MM-DD/stats.parquet`
- Metrics: aggregation lag

### **Observability (Minimal Step 8)**
- Prometheus and Grafana run in Docker Compose
- Ingester, processor, and aggregator also run in Docker Compose and expose `/metrics` on the internal network
- Grafana auto-loads one provisioned dashboard: `arRIval - Minimal Operations`
- Panels: live poll health, delay distribution, system lag, and busiest/worst **proxy** panel
- Note: true per-route “worst/busiest” ranking is not exposed yet via Prometheus labels

### **Delay Calculation (Keep it Stupid)**
```
1. Get bus position (lat/lon)
2. Find nearest station (haversine distance < 100m)
3. Check schedule: "should bus X be at station Y at time T?"
4. If yes → delay = actual_time - scheduled_time
5. If no → ignore (bus between stops)
```

### **Scope Cuts** (keep it minimal)
- No database - just Parquet files
- No web UI - just Grafana dashboards
- No backfilling - only live data
- No schema registry - just JSON
- Static schedule data (load once on startup)
