"""Router report model."""

from __future__ import annotations

from pathlib import Path

from .detectors import collect_actionable_findings
from .model import EpubModel


LEVEL_ORDER = {"error": 0, "warn": 1, "info": 2}


class Report:
  def __init__(self, input_path: Path, workflow_mode: str = "build") -> None:
    self.input = str(input_path)
    self.mode = workflow_mode
    self.input_kind = "unknown"
    self.summary: dict[str, object] = {}
    self.findings: list[dict[str, str]] = []
    self.skills: list[str] = []
    self.skill_levels: dict[str, str] = {}
    self.commands: list[str] = []
    self.tools: dict[str, bool] = {}
    self.model: EpubModel | None = None

  def add_skill(self, name: str, level: str = "info") -> None:
    skill = f"${name}"
    if skill not in self.skills:
      self.skills.append(skill)
    current = self.skill_levels.get(skill)
    if current is None or LEVEL_ORDER[level] < LEVEL_ORDER[current]:
      self.skill_levels[skill] = level

  def add_command(self, command: str) -> None:
    if command not in self.commands:
      self.commands.append(command)

  def findings_by_level(self) -> dict[str, int]:
    counts = {"error": 0, "warn": 0, "info": 0}
    for item in self.findings:
      level = item.get("level", "info")
      counts[level] = counts.get(level, 0) + 1
    return counts

  def as_dict(self) -> dict[str, object]:
    return {
      "input": self.input, "mode": self.mode, "input_kind": self.input_kind,
      "summary": self.summary, "findings": self.findings,
      "findings_by_level": self.findings_by_level(), "recommended_skills": self.skills,
      "suggested_commands": self.commands, "tool_availability": self.tools,
      "actionable_findings": collect_actionable_findings(self.model),
    }


def render_markdown(report: Report) -> str:
  """Render a stable human-readable view of a routing report."""
  data = report.as_dict()
  lines = [
    "# EPUB AI Harness Report",
    "",
    f"- Input: `{data['input']}`",
    f"- Mode: `{data['mode']}`",
    f"- Input kind: `{data['input_kind']}`",
  ]
  if data["summary"]:
    lines.extend(["", "## Summary"])
    for key, value in data["summary"].items():
      lines.append(f"- `{key}`: `{value}`")
  lines.extend(["", "## Findings"])
  for item in data["findings"]:
    path = f" `{item['path']}`" if "path" in item else ""
    lines.append(f"- [{item['level']}]{path} {item['message']}")
  lines.extend(["", "## Recommended Skills"])
  for skill in data["recommended_skills"]:
    lines.append(f"- `{skill}`")
  if data["tool_availability"]:
    lines.extend(["", "## Tool Availability"])
    for tool, available in data["tool_availability"].items():
      lines.append(f"- `{tool}`: `{available}`")
  lines.extend(["", "## Suggested Commands"])
  for command in data["suggested_commands"]:
    lines.append(f"- `{command}`")
  return "\n".join(lines) + "\n"


__all__ = ["LEVEL_ORDER", "Report", "render_markdown"]
