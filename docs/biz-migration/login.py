import json
import warnings
import pytest


from ciphers.aes_ciphers import AESCiphers
from ciphers.bcrypt_ciphers import BCryptCiphers
from client.http_client import HttpSession

# GLOBAL VARIABLES
SECRET_KEY = 'AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'
IV = 'BBBBBBBBBBBBBBBBBBBBBA=='
USER_NAME = ["13312345674"]
PASSWORD = ["861365"]


warnings.filterwarnings('ignore')


@pytest.fixture(scope="session", params=USER_NAME)
def prepare_user_name(request):
    user_name = request.param
    aes = AESCiphers(key=SECRET_KEY, iv=IV)
    user_name = aes.encrypt(user_name)
    yield user_name


@pytest.fixture(scope="session")
def get_app_salt(prepare_user_name):
    user_name = prepare_user_name

    base_url = "https://dev-manhattan.shell.com.cn:9097/jv/crm/jv/auth/v1"
    get_salt_payload = json.dumps({"username": user_name})
    headers = {
        'Content-Type': 'application/json'
    }

    url = base_url + "/get-app-salt"

    http_session = HttpSession()
    response = http_session.post(url=url, data=get_salt_payload, headers=headers, verify=False)
    response = response.json()
    data = response.get("data", {})
    secret_key = data.get("secretKey", "")
    iv = data.get("iv", "")
    salt = data.get("saltStr", "")
    yield user_name, secret_key, iv, salt


@pytest.fixture(scope="session", params=PASSWORD)
def prepare_login_data(get_app_salt, request):
    user_name, secret_key, iv, salt = get_app_salt
    aes = AESCiphers(key=secret_key, iv=iv)
    decrypt_salt = aes.decrypt(salt)
    b_secret_key, b_salt = str(decrypt_salt).split(":")
    assert b_secret_key == secret_key

    b_info = b_salt.split("$")
    b_rounds = int(b_info[2])
    bcrypt_cipher = BCryptCiphers(rounds=b_rounds)
    hashed_password = bcrypt_cipher.encode(password=request.param, salt=b_salt)

    login_info = secret_key + ":" + hashed_password
    login_info = aes.encrypt(login_info)

    yield login_info, secret_key, user_name


@pytest.fixture(scope="session")
def login(prepare_login_data):
    base_url = "https://dev-manhattan.shell.com.cn:9097/jv/jv-adapter/jv/auth/v1"
    url = base_url + "/username-login"

    headers = {
        'Content-Type': 'application/json'
    }

    login_info, secret_key, user_name = prepare_login_data
    login_payload = json.dumps({"loginInfo": login_info, "username": user_name, "secretKey": secret_key})

    http_session = HttpSession()
    response = http_session.post(url=url, data=login_payload, headers=headers, verify=False)
    response = response.json()
    data = response.get("data", {})
    jwt_token = data.get("jwtToken", "")

    yield jwt_token


class TestLogin:
    @pytest.mark.SIT
    def test_login(self, login):
        token = login


if __name__ == '__main__':
    pytest_main_args = [
        "-s",
        "-v",
        "-m SIT or UAT",
        "--disable-warnings",
    ]
    test_case_dirs = [
        "."
    ]
    pytest.main(pytest_main_args + test_case_dirs)