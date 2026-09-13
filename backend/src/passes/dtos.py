from datetime import datetime

from pydantic import BaseModel, EmailStr


# Make sure the name is EXACTLY PassCreateSchema
class PassCreateSchema(BaseModel):
    visitor_name: str
    visitor_email: EmailStr
    host_user_id: str
    valid_until: datetime


class PassScanSchema(BaseModel):
    qr_token: str
    gate_id: str = "Main Gate"
