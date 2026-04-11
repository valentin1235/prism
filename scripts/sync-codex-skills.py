#!/usr/bin/env python3
"""Sync managed Prism Codex skills from the repo source tree into ~/.codex/skills."""

from __future__ import annotations

import argparse
import csv
from dataclasses import dataclass
from pathlib import Path
import shutil
import sys


SKILL_ENTRYPOINT = "SKILL.md"


@dataclass(frozen=True)
class ManagedSkill:
    command_name: str
    source_dir: Path
    install_dir_name: str


def validate_canonical_skill_dir(command_name: str, skill_dir: str, registry_path: Path) -> None:
    normalized_skill_dir = skill_dir.strip()
    if normalized_skill_dir != command_name:
        raise ValueError(
            "Prism shared skill registry drift in "
            f"{registry_path}: command {command_name!r} must use repo source "
            f"'skills/{command_name}/', got 'skills/{normalized_skill_dir}/'"
        )
    if Path(normalized_skill_dir).name != normalized_skill_dir:
        raise ValueError(
            "Prism shared skill registry drift in "
            f"{registry_path}: command {command_name!r} must map to a direct child of repo "
            f"skills/, got {normalized_skill_dir!r}"
        )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Refresh managed Prism Codex skills from the repo skills/ source into the Codex "
            "install root."
        )
    )
    parser.add_argument("--repo-root", required=True)
    parser.add_argument("--registry-path", required=True)
    parser.add_argument("--shared-skills-root")
    parser.add_argument("--target-root", required=True)
    parser.add_argument("--namespace", default="prism-")
    return parser.parse_args()


def load_managed_skills(shared_skills_root: Path, registry_path: Path) -> tuple[ManagedSkill, ...]:
    managed_skills: list[ManagedSkill] = []
    with registry_path.open("r", encoding="utf-8", newline="") as handle:
        reader = csv.reader(handle, delimiter="\t")
        for row in reader:
            if not row:
                continue

            command_name = row[0].strip()
            if not command_name or command_name.startswith("#"):
                continue

            if len(row) != 3:
                raise ValueError(f"Invalid registry row in {registry_path}: {row!r}")

            skill_dir = row[1].strip()
            installed_skill_id = row[2].strip()
            validate_canonical_skill_dir(command_name, skill_dir, registry_path)
            source_dir = (shared_skills_root / skill_dir).resolve()
            skill_md_path = source_dir / SKILL_ENTRYPOINT

            if not source_dir.is_dir():
                raise FileNotFoundError(f"Missing shared Prism skill directory: {source_dir}")
            if not skill_md_path.is_file():
                raise FileNotFoundError(f"Missing shared Prism skill entrypoint: {skill_md_path}")

            managed_skills.append(
                ManagedSkill(
                    command_name=command_name,
                    source_dir=source_dir,
                    install_dir_name=installed_skill_id,
                )
            )

    if not managed_skills:
        raise FileNotFoundError(f"No managed Prism Codex skills found in {registry_path}")

    return tuple(managed_skills)


def remove_existing(path: Path) -> None:
    if path.is_dir() and not path.is_symlink():
        shutil.rmtree(path)
        return
    path.unlink()


def ensure_target_root_is_external(repo_root: Path, target_root: Path) -> None:
    try:
        target_root.relative_to(repo_root)
    except ValueError:
        return

    raise ValueError(
        "Refusing to sync managed Prism skills inside the Prism repo. "
        "Use repo skills/ as the authored source and sync managed installs into ~/.codex/skills only."
    )


def sync_managed_skills(
    managed_skills: tuple[ManagedSkill, ...],
    repo_root: Path,
    target_root: Path,
    namespace: str,
) -> tuple[Path, ...]:
    ensure_target_root_is_external(repo_root, target_root)
    target_root.mkdir(parents=True, exist_ok=True)
    installed_paths: list[Path] = []
    installed_names = {managed_skill.install_dir_name for managed_skill in managed_skills}

    for managed_skill in managed_skills:
        target_path = (target_root / managed_skill.install_dir_name).resolve()
        if target_path == managed_skill.source_dir:
            raise ValueError(
                "Refusing to install managed Prism skills in-place from the repo source tree"
            )

        if target_path.exists():
            remove_existing(target_path)

        shutil.copytree(managed_skill.source_dir, target_path)
        installed_paths.append(target_path)

    for installed_path in sorted(target_root.iterdir()):
        if installed_path.name in installed_names:
            continue
        if installed_path.is_dir() and installed_path.name.startswith(namespace):
            remove_existing(installed_path)

    return tuple(installed_paths)


def main() -> int:
    args = parse_args()
    repo_root = Path(args.repo_root).expanduser().resolve()
    registry_path = Path(args.registry_path).expanduser().resolve()
    shared_skills_root = (
        Path(args.shared_skills_root).expanduser().resolve()
        if args.shared_skills_root
        else (repo_root / "skills").resolve()
    )
    target_root = Path(args.target_root).expanduser().resolve()

    managed_skills = load_managed_skills(shared_skills_root, registry_path)
    installed_paths = sync_managed_skills(managed_skills, repo_root, target_root, args.namespace)

    for installed_path in installed_paths:
        print(installed_path)

    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as exc:  # pragma: no cover - surfaced via shell/go integration tests
        print(f"ERROR: {exc}", file=sys.stderr)
        raise SystemExit(1)
