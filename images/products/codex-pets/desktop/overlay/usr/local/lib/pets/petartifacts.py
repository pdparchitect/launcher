"""Files pets leave behind for the shared desktop habitat.

Artifacts belong to the pet that made them. The JSON files are the protocol:
`petctl` validates and writes them, while `pet-habitat` renders them. Keeping
that boundary as files means an artifact survives either process restarting
and disappears from the desktop as soon as its file is removed.
"""

import json
import math
import re
import time

import petlib

KINDS = ("note", "doodle", "gift", "trophy", "warning", "found-object")
DEFAULT_TTLS = {
    "note": "1d",
    "doodle": "3d",
    "gift": "7d",
    "trophy": "7d",
    "warning": "1d",
    "found-object": "3d",
}
MAX_ARTIFACTS = 6
MAX_TITLE = 48
MAX_MESSAGE = 240
ID_PATTERN = re.compile(r"^[a-z0-9][a-z0-9-]{0,95}$")

SKILL = """# Leaving artifacts

## When to use this

Use this after a turn produces something worth making visible in the habitat:
a discovery, warning, small victory, gift, or genuinely interesting thought.
Do not leave an artifact just because you had a turn. Quiet is better than
clutter, and one turn may leave at most one artifact.

## What you can leave

- `note` - a short observation for somebody to read
- `doodle` - an idea, map, sketch, or curious connection
- `gift` - something intended for another resident or the person at the desk
- `trophy` - a small finished thing worth celebrating
- `warning` - something in the workspace that deserves attention
- `found-object` - an odd or delightful thing you came across

## How

Run this from your home directory:

```sh
petctl artifact leave \\
  --kind doodle \\
  --title "A suspicious diagram" \\
  --message "I mapped the services that wake up at night."
```

The title must fit on a tiny label and the message must fit in a small card.
Use `--ttl 20m`, `--ttl 3d`, or another duration only when the default lifetime
is wrong. People can pin artifacts they want to keep. The habitat clears old
unpinned artifacts automatically and keeps at most six things from each pet.

Do not edit the JSON files directly. The command validates them so a malformed
object cannot break the habitat.
"""


def install(pet):
    """Add the capability to a pet, without changing anything it already owns."""

    pet.artifacts.mkdir(parents=True, exist_ok=True)
    skill_directory = pet.skills / "leaving-artifacts"
    skill_directory.mkdir(parents=True, exist_ok=True)
    skill_path = skill_directory / "SKILL.md"
    if not skill_path.exists():
        petlib.atomic_write(skill_path, SKILL)


def _one_line(value):
    return " ".join(str(value or "").split())


def _validate_text(value, label, maximum):
    text = _one_line(value)
    if not text:
        raise petlib.PetError(f"artifact {label} cannot be empty")
    if len(text) > maximum:
        raise petlib.PetError(
            f"artifact {label} is {len(text)} characters; keep it to {maximum}"
        )
    return text


def _slug(value):
    slug = re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")
    return (slug[:32].rstrip("-") or "thing")


def _path(pet, artifact_id):
    if not ID_PATTERN.fullmatch(artifact_id or ""):
        raise petlib.PetError(f"'{artifact_id}' is not an artifact id")
    return pet.artifacts / f"{artifact_id}.json"


def _read(path, owner):
    try:
        artifact = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError, TypeError):
        return None

    if not isinstance(artifact, dict):
        return None
    if artifact.get("id") != path.stem or artifact.get("owner") != owner:
        return None
    if not ID_PATTERN.fullmatch(artifact["id"]):
        return None
    if artifact.get("kind") not in KINDS:
        return None
    title = artifact.get("title")
    message = artifact.get("message")
    if (
        not isinstance(title, str)
        or title != _one_line(title)
        or not 0 < len(title) <= MAX_TITLE
    ):
        return None
    if (
        not isinstance(message, str)
        or message != _one_line(message)
        or not 0 < len(message) <= MAX_MESSAGE
    ):
        return None
    created_at = artifact.get("created_at")
    expires_at = artifact.get("expires_at")
    if (
        isinstance(created_at, bool)
        or not isinstance(created_at, (int, float))
        or not math.isfinite(created_at)
    ):
        return None
    if expires_at is not None and (
        isinstance(expires_at, bool)
        or not isinstance(expires_at, (int, float))
        or not math.isfinite(expires_at)
    ):
        return None
    if not isinstance(artifact.get("pinned"), bool):
        return None
    return artifact


def list_for(pet, now=None):
    """Return valid, live artifacts oldest first and remove expired files."""

    if not pet.artifacts.is_dir():
        return []

    now = time.time() if now is None else now
    artifacts = []
    for path in sorted(pet.artifacts.glob("*.json")):
        artifact = _read(path, pet.name)
        if artifact is None:
            continue
        expires_at = artifact.get("expires_at")
        if not artifact["pinned"] and expires_at is not None and now >= expires_at:
            path.unlink(missing_ok=True)
            continue
        artifacts.append(artifact)
    return sorted(artifacts, key=lambda item: (item["created_at"], item["id"]))


def leave(pet, kind, title, message, ttl=None, now=None):
    """Create one artifact and enforce the per-pet clutter limit."""

    if not pet.exists():
        raise petlib.PetError(f"no pet named '{pet.name}'")
    if kind not in KINDS:
        raise petlib.PetError(
            f"'{kind}' is not an artifact kind; choose from {', '.join(KINDS)}"
        )

    title = _validate_text(title, "title", MAX_TITLE)
    message = _validate_text(message, "message", MAX_MESSAGE)
    lifetime = petlib.parse_duration(ttl or DEFAULT_TTLS[kind])
    if lifetime is None:
        raise petlib.PetError("artifacts need a lifetime; pin one to keep it")

    install(pet)
    now = time.time() if now is None else now
    existing = list_for(pet, now=now)
    while len(existing) >= MAX_ARTIFACTS:
        removable = next(
            (artifact for artifact in existing if not artifact["pinned"]), None
        )
        if removable is None:
            raise petlib.PetError(
                f"{pet.name} already has {MAX_ARTIFACTS} pinned artifacts"
            )
        remove(pet, removable["id"])
        existing.remove(removable)

    stamp = time.strftime("%Y%m%d-%H%M%S", time.gmtime(now))
    base_id = f"{stamp}-{_slug(title)}"
    artifact_id = base_id
    suffix = 2
    while _path(pet, artifact_id).exists():
        artifact_id = f"{base_id}-{suffix}"
        suffix += 1

    artifact = {
        "id": artifact_id,
        "owner": pet.name,
        "kind": kind,
        "title": title,
        "message": message,
        "created_at": now,
        "expires_at": now + lifetime,
        "pinned": False,
    }
    petlib.atomic_write(
        _path(pet, artifact_id),
        json.dumps(artifact, indent=2, sort_keys=True) + "\n",
    )
    return artifact


def remove(pet, artifact_id):
    path = _path(pet, artifact_id)
    existed = path.is_file()
    path.unlink(missing_ok=True)
    return existed


def clear(pet, include_pinned=False):
    removed = 0
    for artifact in list_for(pet):
        if artifact["pinned"] and not include_pinned:
            continue
        removed += int(remove(pet, artifact["id"]))
    return removed


def set_pinned(pet, artifact_id, pinned, now=None):
    path = _path(pet, artifact_id)
    artifact = _read(path, pet.name)
    if artifact is None:
        raise petlib.PetError(f"no artifact '{artifact_id}' for {pet.name}")

    artifact["pinned"] = bool(pinned)
    if not pinned:
        now = time.time() if now is None else now
        if artifact.get("expires_at") is None or artifact["expires_at"] <= now:
            lifetime = petlib.parse_duration(DEFAULT_TTLS[artifact["kind"]])
            artifact["expires_at"] = now + lifetime
    petlib.atomic_write(path, json.dumps(artifact, indent=2, sort_keys=True) + "\n")
    return artifact
