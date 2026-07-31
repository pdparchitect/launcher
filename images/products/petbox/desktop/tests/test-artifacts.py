#!/usr/bin/env python3

import json
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import petartifacts
import petlib

PRODUCT_DIRECTORY = Path(__file__).resolve().parents[1]
PETCTL = (
    PRODUCT_DIRECTORY
    / "overlay/usr/local/bin/petctl"
)


class ArtifactTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.root = Path(self.temporary.name)
        self.pet = petlib.Pet("jojo", self.root)
        self.pet.directory.mkdir()
        self.pet.config_path.write_text('name = "jojo"\n')

    def tearDown(self):
        self.temporary.cleanup()

    def test_leave_and_expire_artifact(self):
        artifact = petartifacts.leave(
            self.pet,
            kind="doodle",
            title="A suspicious diagram",
            message="I mapped the services that wake up at night.",
            ttl="1h",
            now=1000,
        )

        self.assertEqual(artifact["owner"], "jojo")
        self.assertEqual(artifact["kind"], "doodle")
        self.assertEqual(artifact["expires_at"], 4600)
        self.assertEqual(
            [item["id"] for item in petartifacts.list_for(self.pet, now=4599)],
            [artifact["id"]],
        )
        self.assertEqual(petartifacts.list_for(self.pet, now=4601), [])
        self.assertFalse((self.pet.artifacts / f"{artifact['id']}.json").exists())

    def test_pinned_artifacts_survive_expiry_and_clear(self):
        artifact = petartifacts.leave(
            self.pet,
            kind="gift",
            title="Shiny screw",
            message="This looked important.",
            ttl="1s",
            now=100,
        )
        petartifacts.set_pinned(self.pet, artifact["id"], True, now=100)

        self.assertEqual(len(petartifacts.list_for(self.pet, now=1000)), 1)
        self.assertEqual(petartifacts.clear(self.pet), 0)
        self.assertTrue((self.pet.artifacts / f"{artifact['id']}.json").exists())
        self.assertEqual(petartifacts.clear(self.pet, include_pinned=True), 1)

    def test_limit_removes_oldest_unpinned_artifact(self):
        artifacts = []
        for index in range(petartifacts.MAX_ARTIFACTS):
            artifacts.append(
                petartifacts.leave(
                    self.pet,
                    kind="note",
                    title=f"Note {index}",
                    message="A small thought.",
                    now=100 + index,
                )
            )
        petartifacts.set_pinned(self.pet, artifacts[0]["id"], True, now=200)

        newest = petartifacts.leave(
            self.pet,
            kind="trophy",
            title="Tiny victory",
            message="The test passed.",
            now=300,
        )
        remaining = {
            artifact["id"] for artifact in petartifacts.list_for(self.pet, now=301)
        }

        self.assertEqual(len(remaining), petartifacts.MAX_ARTIFACTS)
        self.assertIn(artifacts[0]["id"], remaining)
        self.assertNotIn(artifacts[1]["id"], remaining)
        self.assertIn(newest["id"], remaining)

    def test_rejects_invalid_artifacts(self):
        with self.assertRaises(petlib.PetError):
            petartifacts.leave(
                self.pet,
                kind="spaceship",
                title="No",
                message="Unknown kinds are not renderable.",
            )
        with self.assertRaises(petlib.PetError):
            petartifacts.leave(
                self.pet,
                kind="note",
                title="",
                message="Titles are required.",
            )
        with self.assertRaises(petlib.PetError):
            petartifacts.leave(
                self.pet,
                kind="note",
                title="Too long lived",
                message="This duration is invalid.",
                ttl="forever-ish",
            )

        self.pet.artifacts.mkdir()
        malformed = {
            "id": "unsafe",
            "owner": "jojo",
            "kind": "note",
            "title": "x" * (petartifacts.MAX_TITLE + 1),
            "message": "A hand-edited file.",
            "created_at": 100,
            "expires_at": 200,
            "pinned": False,
        }
        (self.pet.artifacts / "unsafe.json").write_text(json.dumps(malformed))
        self.assertEqual(petartifacts.list_for(self.pet, now=101), [])

    def test_install_adds_the_skill_without_overwriting_it(self):
        petartifacts.install(self.pet)
        skill = self.pet.skills / "leaving-artifacts" / "SKILL.md"
        self.assertEqual(
            self.pet.skills,
            self.pet.directory / ".agents" / "skills",
        )
        self.assertIn("name: leaving-artifacts", skill.read_text())
        self.assertIn("petctl artifact leave", skill.read_text())

        skill.write_text("Jojo's own rules.\n")
        petartifacts.install(self.pet)
        self.assertEqual(skill.read_text(), "Jojo's own rules.\n")

    def test_new_pet_scaffolds_discoverable_skills(self):
        environment = dict(os.environ)
        environment["PETS_ROOT"] = str(self.root)

        subprocess.run(
            [
                sys.executable,
                str(PETCTL),
                "new",
                "pip",
                "--species",
                "blob",
                "--colour",
                "amber",
                "--heartbeat",
                "off",
            ],
            check=True,
            capture_output=True,
            text=True,
            env=environment,
        )

        pet = petlib.Pet("pip", self.root)
        self.assertTrue(
            (pet.skills / "keeping-a-journal" / "SKILL.md").is_file()
        )
        self.assertTrue(
            (pet.skills / "leaving-artifacts" / "SKILL.md").is_file()
        )
        self.assertFalse((pet.directory / "skills").exists())

    def test_petctl_can_leave_and_list_as_the_current_pet(self):
        environment = dict(os.environ)
        environment["PETS_ROOT"] = str(self.root)
        environment["PET_NAME"] = self.pet.name

        left = subprocess.run(
            [
                sys.executable,
                str(PETCTL),
                "artifact",
                "leave",
                "--kind",
                "warning",
                "--title",
                "Mind the wire",
                "--message",
                "It is carrying more current than it looks.",
            ],
            check=True,
            capture_output=True,
            text=True,
            env=environment,
        )
        listed = subprocess.run(
            [sys.executable, str(PETCTL), "artifact", "list", self.pet.name],
            check=True,
            capture_output=True,
            text=True,
            env=environment,
        )

        self.assertIn("jojo left warning", left.stdout)
        self.assertIn("Mind the wire", listed.stdout)

if __name__ == "__main__":
    unittest.main()
