import os

import urllib3


class ModrinthClient:
    METHOD_GET: str = 'GET'
    METHOD_POST: str = 'POST'
    MODRINTH_API_URL: str = 'https://api.modrinth.com/v2/'

    client: urllib3.PoolManager | None

    def __init__(self) -> None:
        self.client = None

    def get_client(self) -> urllib3.PoolManager:
        if self.client:
            return self.client
        self.client = urllib3.PoolManager(
            timeout=urllib3.Timeout(connect=2.0, read=2.0)
        )
        return self.client

    def make_request(self, method: str, uri: str, auth: bool = False,
                     binary: bool = False) -> urllib3.response.BaseHTTPResponse | None:
        if binary is False:
            url = f'{self.MODRINTH_API_URL + uri}'
        else:
            url = f'{uri}'
        client = self.get_client()
        headers = urllib3.HTTPHeaderDict()
        headers.add('User-Agent', os.getenv('USERAGENT'))
        if auth:
            headers.add('Authorization', os.getenv('MODRINTH_API_KEY'))
        if not binary:
            headers.add('Content-Type', 'application/json')
        if binary:
            headers.add('Accept', 'application/octet-stream')
        response = client.request(method, url, headers=headers)
        if binary is False and response.status != 200:
            print(f'[red]Error: {response.status}[/red]')
            return
        return response

    def get_follows(self):
        return self.make_request(self.METHOD_GET, 'user/snek/follows', auth=True).json()

    def get_project_versions(self, slug: str):
        loader = os.getenv('MINECRAFT_LOADER', 'fabric ')
        game_version = os.getenv('MINECRAFT_VERSION')
        url = f'project/{slug}/version?game_versions=["{game_version}"]&loaders=["{loader}"]'
        return self.make_request(self.METHOD_GET, url, auth=True).json()

    def get_project(self, slug: str):
        url = f'project/{slug}'
        return self.make_request(self.METHOD_GET, url, auth=True).json()

    def download_mod_file(self, filename: str, url: str):
        response = self.make_request(self.METHOD_GET, url, binary=True)
        print(response.data)
        exit()
        with open(f'mods/{filename}', 'wb') as file:
            file.write(response.data)
