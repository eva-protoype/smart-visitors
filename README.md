# Smart Card for Visitors: Centralized Access System

The **Smart Card for Visitors** is a centralized visitor identity verification and registration system[cite: 1]. This backend API handles automated visitor registration, digital pass issuance, hardware gate verification, and access auditing across facilities[cite: 1].

Built for high throughput, the system replaces vulnerable manual logbooks with secure, time-limited digital credentials and transparent entry auditing[cite: 1].

##  Tech Stack

* **Core Framework:** [FastAPI](https://fastapi.tiangolo.com/) (Python 3.11+) for high-concurrency, asynchronous API routes[cite: 1].
* **Database & ORM:** SQLite (dev) / PostgreSQL (prod) with SQLAlchemy[cite: 1].
* **Data Validation:** Pydantic v2 for strict type checking and request payload validation[cite: 1].
* **Machine Learning (WIP):** `scikit-learn` for anomaly detection & LLM integration for Text-to-SQL analytics.

##  Core Features

### Phase 1: MVP Setup (Completed)
* **Role-Based Access Control (RBAC):** Distinct database permissions for visitors, security guards, host personnel, and system administrators[cite: 1].
* **Pass Credential Management:** Secure generation of time-limited QR visitor passes[cite: 1].
* **Access Auditing Workflow:** Persistent logging of every gate scan attempt, capturing entry outcomes and validation statuses[cite: 1].

### Phase 2: AI Integrations (In Progress)
* **Anomaly Detection Engine:** A lightweight `IsolationForest` model to flag suspicious, off-hours, or rapid multi-gate entry attempts.
* **Natural Language Queries:** A Text-to-SQL LLM assistant allowing admins to query access logs in plain English.

##  Local Development Setup

### 1. Clone the repository
```bash
git clone [https://github.com/eva-protoype/smart-visitors.git](https://github.com/eva-protoype/smart-visitors.git)
cd smart-visitors/backend

### virtual environment setup
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -r requirements.txt