from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    DB_CONNECTION: str = "sqlite:///./smart_visitors.db"

    model_config = SettingsConfigDict(env_file=".env", extra="ignore")


settings = Settings()
