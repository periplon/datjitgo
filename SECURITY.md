# Security Policy

## Supported Versions

Only the latest tagged release is supported with security fixes.

## Reporting a Vulnerability

Please email **joanmarc.carbo@gmail.com** with the details of the issue.

- Do **not** file public GitHub issues for security bugs.
- You can expect an acknowledgement within **7 days**.
- We aim for a coordinated disclosure window of up to **90 days** between
  private report and public disclosure.

## Scope

In scope:

- The `datjitgo` library.
- The `datjit` CLI in [`cmd/datjit`](cmd/datjit).
- The embedded corpus shipped with the module.

## Out of Scope

- Third-party LLM providers wired in by users (OpenAI, Ollama, vLLM, LM
  Studio, etc.). Report security issues to those vendors directly.
- Downstream applications that embed `datjitgo`.
