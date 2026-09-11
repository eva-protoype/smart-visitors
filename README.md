# Smart Card for Visitors 🏷️🚪
> **Centralized Visitor Identity Verification & Registration System**

An enterprise-grade, real-time access control and visitor management platform built to replace traditional, error-prone paper logbooks with dynamic digital passes, automated gate verification, and real-time host alerting[cite: 1].

---

## 🚀 Key Features

* **Multi-Protocol Verification:** Supports dynamic QR Codes, NFC, RFID, and Barcode passes for high-speed gate authentication[cite: 1].
* **Real-Time Host Alerts:** Instant notifications delivered to hosts via WebSockets, SMS, and Email upon visitor arrival[cite: 1].
* **Sub-Second Validation Engine:** Asynchronous pipeline capable of handling 500+ concurrent requests with sub-2-second 99th percentile latency.
* **Gate Occupancy Tracking:** Live metrics, dwell-time analytics, and automated overstay alerts calculated via Redis pub/sub.
* **Role-Based Access Control (RBAC):** Granular permissions designed for Visitors, Guards, Hosts, and System Administrators[cite: 1].
* **Tamper-Proof Audit Logging:** Complete timestamp chains for every entry event with 90-day retention support.

---

## 🛠️ Tech Stack

### Frontend
* **Framework:** Next.js 15.2.4 (App Router, Server-Side Rendering)
* **UI Library:** React 18.3.1
* **Styling:** Tailwind CSS
* **Language:** TypeScript 5.x
* **Hardware Integration:** HTML5 Canvas & Web Camera API (`jsQR`)

### Backend
* **API Engine:** FastAPI 0.115+ (Python 3.11+)[cite: 1]
* **Relational Database:** PostgreSQL 16+
* **Cache & Message Broker:** Redis 7.x
* **Authentication:** JWT (Stateless) with Bcrypt password hashing (Cost Factor 12)

---

## ⚡ System Architecture & Verification Pipeline

The verification engine processes pass checks through a 4-stage pipeline:

1. **Registration & Validation:** Scans credentials, validates format, assigns UUID v4 event IDs, and queues via Redis.
2. **Pre-Processing:** Fetches access policies from PostgreSQL, decrypts payload using AES-256-GCM, and checks session bounds.
3. **Execution Engine:** Validates zone permissions and single-entry constraints, returning access decisions in under 200ms.
4. **Post-Processing:** Log persistence, real-time gate occupancy recalculation, and WebSocket host notifications.

---

## 📦 Getting Started

### Prerequisites

* **Node.js:** `v20.x LTS`
* **Python:** `v3.11+`
* **Docker Engine:** `v24.0+` & Docker Compose
* **PostgreSQL:** `v16+`
* **Redis:** `v7.x`

### Local Setup

1. **Clone the repository:**
   ```bash
   git clone [https://github.com/your-username/smart-card-visitors.git](https://github.com/your-username/smart-card-visitors.git)
   cd smart-card-visitors
