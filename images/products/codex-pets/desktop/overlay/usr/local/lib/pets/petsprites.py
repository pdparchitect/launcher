"""Pixel art for the desktop pets.

Every sprite is a rectangular block of characters, one character per pixel,
authored at 16x16 and scaled by an integer factor at draw time. Keeping the
art as text rather than as PNGs is the point: a species is readable in a diff,
a new one is a paste away, and there is no asset pipeline between the file and
the screen.

Only two frames per species are drawn by hand - `idle` and `sleep`. Everything
else is derived:

    walk    idle bobbed one pixel up and down, with the two foot rows stepped
            left and right, which is what reads as walking at this size
    blink   the eye pixels darkened to the outline colour

Deriving the walk cycle is what keeps a species down to two frames of art. It
also means a species contributed later animates correctly without its author
having to match anyone else's timing.

Character map, shared by every species:

    .   transparent          w   eye white
    k   outline / shadow     e   pupil
    b   body                 a   accent (beak, cheeks, feet, lights)
    l   body light
    d   body dark
"""

TRANSPARENT = "."

# The palette every colourway shares. Pets differ in body colour, not in the
# colour of their outline or their eyes, which is what makes a row of them read
# as one species of thing rather than as unrelated clip art.
COMMON_COLOURS = {
    "k": "#0B0C1A",
    "w": "#F2F5FF",
    "e": "#0B0C1A",
}

# Body colourways. A pet picks one in pet.toml; without a choice it is assigned
# from a hash of its name, so two pets created in a row look different.
COLOURWAYS = {
    "mint": {"b": "#3FBF7F", "l": "#63E29E", "d": "#2A8A5C", "a": "#F5C542"},
    "berry": {"b": "#D2568C", "l": "#F07FAE", "d": "#98305F", "a": "#FFE08A"},
    "amber": {"b": "#E8913C", "l": "#FFC061", "d": "#B0632A", "a": "#FFF2B0"},
    "cobalt": {"b": "#4A7BE8", "l": "#7BA6FF", "d": "#2E4FA8", "a": "#9BE8FF"},
    "violet": {"b": "#8B6BE0", "l": "#B196FF", "d": "#5D45A0", "a": "#FFD36E"},
    "slate": {"b": "#7E8AA8", "l": "#A9B5D0", "d": "#4E586F", "a": "#F5C542"},
}

COLOURWAY_NAMES = sorted(COLOURWAYS)


def _rows(art):
    """Split authored art into rows, dropping the leading blank line."""

    return [row for row in art.strip("\n").split("\n")]


# Each species is authored facing the viewer. Direction of travel is expressed
# by mirroring at draw time, so no species needs a second set of art.
SPECIES = {
    "blob": {
        "description": "a slime that has opinions",
        "idle": _rows(
            """
................
................
......kkkk......
....kkllllkk....
...kllllllllk...
..kllllllllllk..
..kllbbbbbbllk..
.kllbwwbbwwbllk.
.klbbweebweebbk.
.klbbbbbbbbbbbk.
.kbbbaabbbaabbk.
.kbbbbbbbbbbbbk.
.kbbbbbbbbbbbbk.
..kddddddddddk..
...kkkkkkkkkk...
................
"""
        ),
        "sleep": _rows(
            """
................
................
................
................
................
.....kkkkkk.....
...kkllllllkk...
..kllllllllllk..
.kllbbbbbbbbllk.
.klbkkbbbbkkbbk.
kbbbbbbbbbbbbbbk
kbbbaabbbbaabbbk
kbbbbbbbbbbbbbbk
.kddddddddddddk.
..kkkkkkkkkkkk..
................
"""
        ),
    },
    "cat": {
        "description": "aloof, prints things on your terminal",
        "idle": _rows(
            """
................
...k........k...
..kak......kak..
..kabkkkkkkbak..
..kbbbbbbbbbbk..
..kbwebbbbwebk..
..kbbbbabbbbbk..
..kbbkbbbbkbbk..
...kbbbbbbbbk...
...kbbbbbbbbk...
..kbbbbbbbbbbk..
..kbbbbbbbbbbk..
..kbbbbbbbbbbk..
..kbkkbbbbkkbk..
..kkk.kkkk.kkk..
................
"""
        ),
        "sleep": _rows(
            """
................
................
................
................
...k........k...
..kak......kak..
..kabkkkkkkbak..
..kbbbbbbbbbbk..
..kbkkbbbbkkbk..
..kbbbbabbbbbk..
.kbbbbbbbbbbbbk.
.kbbbbbbbbbbbbk.
.kbbbbbbbbbbbbk.
..kddddddddddk..
...kkkkkkkkkk...
................
"""
        ),
    },
    "bird": {
        "description": "small, fast, forgets things",
        "idle": _rows(
            """
................
......kkkk......
....kkllllkk....
...kllllllllk...
..kllllllllllk..
..klwebbbbwelk..
..kllllaallllk..
.kbllllllllllbk.
.kbbllllllllbbk.
.kbbbllllllbbbk.
.kbbbbllllbbbbk.
..kbbbbbbbbbbk..
...kkbbbbbbkk...
.....kkkkkk.....
....kaak.kaak...
.....kk...kk....
"""
        ),
        "sleep": _rows(
            """
................
................
................
......kkkk......
....kkllllkk....
...kllllllllk...
..kllllllllllk..
..klkkllllkklk..
..kllllaallllk..
.kbllllllllllbk.
.kbbllllllllbbk.
.kbbbbllllbbbbk.
..kbbbbbbbbbbk..
...kkbbbbbbkk...
....kaakkaak....
................
"""
        ),
    },
    "bot": {
        "description": "runs the errands nobody else wants",
        "idle": _rows(
            """
.......kk.......
.......ak.......
.......kk.......
..kkkkkkkkkkkk..
..kllllllllllk..
..klwebbbbwelk..
..kllllllllllk..
..klkkllllkklk..
..kkkkkkkkkkkk..
...kbbbbbbbbk...
.kkkbbaaaabbkkk.
.kbkbbbbbbbbkbk.
..kkbbbbbbbbkk..
...kbbbbbbbbk...
...kkk....kkk...
................
"""
        ),
        "sleep": _rows(
            """
................
................
.......kk.......
.......dk.......
.......kk.......
..kkkkkkkkkkkk..
..kllllllllllk..
..klkkllllkklk..
..kllllllllllk..
..kkkkkkkkkkkk..
...kbbbbbbbbk...
.kkkbbddddbbkkk.
.kbkbbbbbbbbkbk.
...kbbbbbbbbk...
...kkk....kkk...
................
"""
        ),
    },
}

SPECIES_NAMES = sorted(SPECIES)


def validate():
    """Assert every authored frame is a rectangle of known characters.

    Called by the image source tests. A ragged row would otherwise surface as a
    pet with a torn silhouette at run time, inside a session, where nobody is
    looking at this file.
    """

    known = set(TRANSPARENT) | set(COMMON_COLOURS) | {"b", "l", "d", "a"}

    for species, frames in SPECIES.items():
        for frame in ("idle", "sleep"):
            rows = frames[frame]
            width = len(rows[0])
            for index, row in enumerate(rows):
                if len(row) != width:
                    raise ValueError(
                        f"{species}/{frame} row {index} is {len(row)} wide, "
                        f"but row 0 is {width}"
                    )
                unknown = set(row) - known
                if unknown:
                    raise ValueError(
                        f"{species}/{frame} row {index} uses unknown pixels: "
                        f"{''.join(sorted(unknown))}"
                    )

    for name, colours in COLOURWAYS.items():
        missing = {"b", "l", "d", "a"} - set(colours)
        if missing:
            raise ValueError(f"colourway {name} is missing {sorted(missing)}")

    return True


def palette(colourway):
    """Resolve a colourway name to a full character-to-colour map."""

    colours = COLOURWAYS.get(colourway) or COLOURWAYS["mint"]
    return {**COMMON_COLOURS, **colours}


def colourway_for(name):
    """Pick a stable colourway from a pet's name.

    Deliberately not random: the same pet is the same colour after a restart,
    without the colour having to be written anywhere.
    """

    digest = sum((index + 1) * byte for index, byte in enumerate(name.encode()))
    return COLOURWAY_NAMES[digest % len(COLOURWAY_NAMES)]


def _mirror(rows):
    return [row[::-1] for row in rows]


def _bob(rows, offset):
    """Shift a frame vertically, padding with transparent rows.

    A positive offset moves the sprite up. The frame keeps its dimensions, so
    the window never has to be resized mid-animation.
    """

    width = len(rows[0])
    blank = TRANSPARENT * width

    if offset > 0:
        return rows[offset:] + [blank] * offset
    if offset < 0:
        return [blank] * -offset + rows[:offset]
    return list(rows)


def _step(rows, direction):
    """Shift the two lowest non-empty rows sideways.

    This is the whole walk cycle. At 16 pixels tall the feet are one or two
    rows, and moving them a pixel against a still body is what the eye reads as
    a step - a full second pose per species is not worth authoring.
    """

    stepped = list(rows)
    occupied = [
        index for index, row in enumerate(rows) if set(row) != {TRANSPARENT}
    ]
    if not occupied:
        return stepped

    width = len(rows[0])
    for index in occupied[-2:]:
        row = rows[index]
        if direction > 0:
            stepped[index] = (TRANSPARENT + row)[:width]
        else:
            stepped[index] = (row + TRANSPARENT)[1:]

    return stepped


def _close_eyes(rows):
    return [row.replace("w", "k").replace("e", "k") for row in rows]


def frames_for(species, motion):
    """The animation cycle for a species in one of its motions.

    Returns a list of frames; the caller advances through it on its own clock.
    Motions are `idle`, `walk`, `sleep`, `blink` and `think`.
    """

    art = SPECIES.get(species) or SPECIES["blob"]
    idle = art["idle"]

    if motion == "sleep":
        return [art["sleep"], _bob(art["sleep"], -1)]
    if motion == "blink":
        return [_close_eyes(idle)]
    if motion == "walk":
        return [
            _step(idle, 1),
            _step(_bob(idle, 1), -1),
            _step(idle, -1),
            _step(_bob(idle, 1), 1),
        ]
    if motion == "think":
        # A pet with a turn running rocks in place. Movement without travel is
        # what separates "busy" from "wandering" at a glance across the desk.
        return [idle, _bob(idle, 1), idle, _bob(idle, 1)]

    return [idle]


def flip(rows):
    """Mirror a frame so a species can face either way from one drawing."""

    return _mirror(rows)


if __name__ == "__main__":  # pragma: no cover - a preview for authoring
    import sys

    validate()
    wanted = sys.argv[1:] or SPECIES_NAMES
    for species in wanted:
        for motion in ("idle", "walk", "sleep"):
            for index, frame in enumerate(frames_for(species, motion)):
                print(f"{species} {motion} {index}")
                print("\n".join(row.replace(".", " ") for row in frame))
                print()
