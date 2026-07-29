"""Shared model for the Codex Pets programs.

A pet is a directory, not a process:

    /workspace/pets/<name>/
        AGENTS.md      what Codex reads before every turn - the pet's character
        pet.toml       identity and schedule: species, colour, heartbeat
        skills/        one directory per skill, each with a SKILL.md
        memory/        what the pet chooses to remember between turns
        journal/       dated entries the pet writes as it works
        inbox/         messages waiting for it
        .state/        runtime only: state.json, triggers, turn transcripts

Three programs share this module. `petd` supervises, `pet-run` runs one turn
through `codex exec`, and `pet-sprite` draws the pet on the desktop. They never
talk to each other directly - the pet's directory is the whole protocol, which
is why a pet survives a session restart and why `cat` is a sufficient debugger.

Turn state is deliberately the only file the supervisor writes into a running
pet's directory. Everything else under the pet belongs to the pet.
"""

import json
import os
import re
import time
import tomllib
from pathlib import Path

import petsprites

PETS_ROOT = Path(os.environ.get("PETS_ROOT", "/workspace/pets"))

# The pet is idle, thinking (a turn is running), or asleep (disabled, or the
# harness has nothing to run it with). What it last said is carried separately
# from the activity so a bubble can outlive the turn that produced it.
ACTIVITIES = ("idle", "think", "sleep", "error")

# How long a pet's last line stays in a speech bubble.
SPEECH_SECONDS = 18

NAME_PATTERN = re.compile(r"^[a-z][a-z0-9-]{0,23}$")

DEFAULT_HEARTBEAT = "20m"

# Codex' own sandbox modes are implemented with bubblewrap, which needs
# unprivileged user namespaces. A container does not have them under Docker's
# default profile, so `workspace-write` fails there before a pet runs a single
# command. The container is already the boundary - it is a disposable desktop
# with one workspace volume - so pets run unsandboxed inside it by default and
# `petctl doctor` reports whether the tighter modes are available here.
DEFAULT_SANDBOX = "danger-full-access"


class PetError(Exception):
    """A problem a person can fix, reported without a traceback."""


def validate_name(name):
    """Names become directory names and window titles, so keep them boring."""

    if not NAME_PATTERN.match(name or ""):
        raise PetError(
            f"'{name}' is not a usable pet name. Use lower-case letters, "
            "digits and dashes, starting with a letter, up to 24 characters."
        )
    return name


def parse_duration(value, default=None):
    """Parse `45s`, `20m`, `2h`, `off`. Returns seconds, or None for off."""

    if value is None:
        return default
    text = str(value).strip().lower()
    if text in ("off", "never", "none", "0"):
        return None

    match = re.fullmatch(r"(\d+)\s*([smh]?)", text)
    if not match:
        raise PetError(f"'{value}' is not a duration - try 30s, 20m or 2h")

    amount = int(match.group(1))
    return amount * {"": 60, "s": 1, "m": 60, "h": 3600}[match.group(2)]


def format_duration(seconds):
    if seconds is None:
        return "off"
    if seconds % 3600 == 0:
        return f"{seconds // 3600}h"
    if seconds % 60 == 0:
        return f"{seconds // 60}m"
    return f"{seconds}s"


def atomic_write(path, text):
    """Write through a temporary file in the same directory.

    The sprite polls state.json several times a second. A partial read of a
    half-written file is a crashed pet, so every writer here renames into
    place instead of truncating.
    """

    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    temporary.write_text(text)
    temporary.replace(path)


def runtime_directory():
    """Where the session's throwaway files go.

    Not under the pen: a pid is true for one boot of one session, and the pen
    is a persistent volume that outlives both.
    """

    directory = Path(
        os.environ.get("XDG_RUNTIME_DIR") or f"/tmp/pets-{os.getuid()}"
    )
    directory.mkdir(parents=True, exist_ok=True)
    return directory


def daemon_pidfile():
    return runtime_directory() / "petd.pid"


def daemon_pid():
    """The supervisor's pid, or None if it is not running.

    The pidfile is the fast path. It lives under the session's runtime
    directory, so a shell that arrived some other way - `docker exec`, a cron
    job - would not find it and would wrongly report a healthy pen as dead;
    hence the scan.
    """

    try:
        pid = int(daemon_pidfile().read_text().strip())
        if Path(f"/proc/{pid}").exists():
            return pid
    except (OSError, ValueError):
        pass

    for entry in Path("/proc").iterdir():
        if not entry.name.isdigit():
            continue
        try:
            command = (entry / "cmdline").read_bytes().split(b"\0")
        except OSError:
            continue
        if any(part.endswith(b"/petd") or part == b"petd" for part in command):
            return int(entry.name)

    return None


def codex_home():
    return Path(os.environ.get("CODEX_HOME", "/home/agent/.codex"))


def codex_is_signed_in():
    """Whether Codex has credentials, without spawning Codex to find out.

    `codex login status` is authoritative but costs a process launch, and the
    supervisor asks this question on every pass. Both the API-key and the
    ChatGPT sign-in paths land in the same file.
    """

    return (codex_home() / "auth.json").exists()


class Pet:
    """One pet directory, with the accessors the three programs need."""

    def __init__(self, name, root=None):
        self.name = name
        self.root = Path(root or PETS_ROOT)

    # Layout ---------------------------------------------------------------

    @property
    def directory(self):
        return self.root / self.name

    @property
    def config_path(self):
        return self.directory / "pet.toml"

    @property
    def agents_path(self):
        return self.directory / "AGENTS.md"

    @property
    def state_directory(self):
        return self.directory / ".state"

    @property
    def state_path(self):
        return self.state_directory / "state.json"

    @property
    def trigger_directory(self):
        return self.state_directory / "triggers"

    @property
    def turns_directory(self):
        return self.state_directory / "turns"

    @property
    def inbox(self):
        return self.directory / "inbox"

    @property
    def journal(self):
        return self.directory / "journal"

    @property
    def memory(self):
        return self.directory / "memory"

    @property
    def skills(self):
        return self.directory / "skills"

    def exists(self):
        return self.config_path.is_file()

    # Configuration --------------------------------------------------------

    def config(self):
        """Read pet.toml.

        Read on every access rather than cached: editing pet.toml by hand and
        watching the pet change colour a second later is the intended way to
        work on a pet, and no caller is hot enough for the read to matter.
        """

        try:
            with self.config_path.open("rb") as handle:
                return tomllib.load(handle)
        except FileNotFoundError:
            raise PetError(f"no pet named '{self.name}' in {self.root}")
        except tomllib.TOMLDecodeError as error:
            raise PetError(f"{self.config_path} is not valid TOML: {error}")

    @property
    def species(self):
        species = self.config().get("species", "blob")
        return species if species in petsprites.SPECIES else "blob"

    @property
    def colourway(self):
        chosen = self.config().get("colour") or self.config().get("color")
        if chosen in petsprites.COLOURWAYS:
            return chosen
        return petsprites.colourway_for(self.name)

    @property
    def enabled(self):
        return bool(self.config().get("enabled", True))

    @property
    def heartbeat_seconds(self):
        return parse_duration(self.config().get("heartbeat"), parse_duration(DEFAULT_HEARTBEAT))

    @property
    def jitter_seconds(self):
        return parse_duration(self.config().get("jitter"), 120) or 0

    @property
    def sandbox(self):
        return self.config().get("sandbox", DEFAULT_SANDBOX)

    @property
    def model(self):
        return self.config().get("model") or ""

    @property
    def turn_timeout(self):
        return parse_duration(self.config().get("timeout"), 600) or 600

    # Runtime state --------------------------------------------------------

    def read_state(self):
        try:
            state = json.loads(self.state_path.read_text())
        except (FileNotFoundError, json.JSONDecodeError):
            state = {}

        state.setdefault("activity", "idle")
        state.setdefault("detail", "")
        state.setdefault("message", "")
        state.setdefault("message_at", 0)
        state.setdefault("updated", 0)
        return state

    def write_state(self, **changes):
        state = self.read_state()
        state.update(changes)
        state["updated"] = time.time()
        atomic_write(self.state_path, json.dumps(state, indent=2) + "\n")
        return state

    def ensure_state(self, activity, detail=""):
        """Write state only when it would actually change.

        The supervisor passes over every pet twice a second. Rewriting an
        unchanged state.json on each pass would churn the workspace volume and
        make the file's timestamp meaningless to anything watching it.
        """

        state = self.read_state()
        if state["activity"] == activity and state["detail"] == detail:
            return state
        return self.write_state(activity=activity, detail=detail)

    def say(self, message):
        """Record a line for the pet's speech bubble."""

        message = " ".join((message or "").split())
        return self.write_state(message=message, message_at=time.time())

    def speaking(self, now=None):
        """The line to show right now, if the pet said something recently."""

        state = self.read_state()
        if not state["message"]:
            return ""
        if (now or time.time()) - state["message_at"] > SPEECH_SECONDS:
            return ""
        return state["message"]

    # Triggers -------------------------------------------------------------

    def enqueue(self, kind, payload=""):
        """Ask the supervisor to run a turn.

        Triggers are files rather than a socket so that anything - a menu
        entry, a click on the sprite, a cron job, another pet - can start a
        turn without knowing whether the supervisor is up yet.
        """

        self.trigger_directory.mkdir(parents=True, exist_ok=True)
        stamp = f"{time.time():.6f}"
        atomic_write(
            self.trigger_directory / f"{stamp}.json",
            json.dumps({"kind": kind, "payload": payload, "at": time.time()}),
        )

    def take_triggers(self):
        """Claim every pending trigger, oldest first."""

        if not self.trigger_directory.is_dir():
            return []

        claimed = []
        for path in sorted(self.trigger_directory.glob("*.json")):
            try:
                trigger = json.loads(path.read_text())
            except (OSError, json.JSONDecodeError):
                trigger = None
            try:
                path.unlink()
            except OSError:
                continue
            if trigger:
                claimed.append(trigger)
        return claimed


def iter_pets(root=None):
    """Every pet in the pen, in a stable order."""

    root = Path(root or PETS_ROOT)
    if not root.is_dir():
        return []

    pets = []
    for entry in sorted(root.iterdir()):
        if entry.is_dir() and (entry / "pet.toml").is_file():
            pets.append(Pet(entry.name, root))
    return pets


def paused(root=None):
    """A pen-wide pause. Sprites keep walking; nothing spends a token."""

    return (Path(root or PETS_ROOT) / ".paused").exists()
