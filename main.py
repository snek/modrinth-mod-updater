import os.path
from peewee import *
from dotenv import load_dotenv
from rich import print
from modrinth_client import ModrinthClient

client = ModrinthClient()
load_dotenv()

db = SqliteDatabase('mods.db')


class BaseModel(Model):
    class Meta:
        database = db


class Mod(BaseModel):
    id = AutoField()
    title = CharField(unique=True)
    slug = CharField(unique=True)
    filename = CharField()


db.connect()
db.create_tables([Mod])


def mod_file_exists(file) -> bool:
    return os.path.isfile(f'mods/{file}')


def mod_file_delete(file) -> None:
    os.remove(f'mods/{file}')


def check_game_version(proj: dict, version: str) -> bool:
    if version not in proj['game_versions']:
        print(f'[red]Did not find {proj["title"]} for game version {version}[/red]')
        return False
    return True


def check_installation_type(proj: dict, installation: str) -> bool:
    if proj[installation] == 'unsupported':
        return False
    return True


def check_project_versions(ver: list) -> bool:
    if len(ver) == 0 or ver[0] is None:
        return False
    return True


if __name__ == '__main__':
    game_version = os.getenv('MINECRAFT_VERSION')
    follows = client.get_follows()
    print(f'[green]Found {len(follows)} mods to check[/green]')
    for project in follows:
        if not check_game_version(project, game_version):
            continue

        installation_type = os.getenv('MINECRAFT_INSTALLATION')
        if not check_installation_type(project, installation_type):
            continue

        versions = client.get_project_versions(project['slug'])
        filename = versions[0]['files'][0]['filename']
        url = versions[0]['files'][0]['url']

        if not check_project_versions(versions):
            print(f'[red]Did not find {project["title"]} for game version {game_version}[/red]')
            continue

        mod = Mod.get_or_none(Mod.slug == project['slug'])
        if mod is None:
            Mod.create(title=project['title'], slug=project['slug'], filename=filename)

        if mod_file_exists(filename):
            print(f'[yellow] Already downloaded {filename}[/yellow]')
            continue

        if mod.filename != filename:
            print(f'[green]New version of {project["title"]} found, deleting old version[/green]')
            mod_file_delete(mod.filename)
            Mod.update(title=project['title'], slug=project['slug'], filename=filename). \
                where(Mod.slug == project['slug']). \
                execute()

        print(f'[green]Downloading {url}[/green]')
        client.download_mod_file(filename, url)
