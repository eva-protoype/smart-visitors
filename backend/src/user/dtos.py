from pydantic import BaseModel, EmailStr

from src.user.models import UserRole


class UserCreateSchema(BaseModel):
    username: str
    aadhar_number: str
    email: EmailStr
    role: UserRole = UserRole.HOST
