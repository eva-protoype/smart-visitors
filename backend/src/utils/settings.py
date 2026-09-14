from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    DB_CONNECTION: str = "sqlite:///./smart_visitors.db"

    # Step 2 (Phase 2 AI) settings
    ANOMALY_CONTAMINATION: float = 0.1
    AI_ARTIFACT_DIR: str = "./artifacts"
    AI_MAX_QUERY_ROWS: int = 50
    LLM_PROVIDER: str = "none"  # none | openai | local (hook for Phase 3)

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")


settings = Settings()
