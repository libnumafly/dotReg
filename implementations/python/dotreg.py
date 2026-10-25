"""Dependency-free dotReg syntax and common string decoding (Python 3.10+)."""

from __future__ import annotations

import argparse
from dataclasses import dataclass
import json
import sys
import unicodedata
from typing import BinaryIO, Callable


class ParseError(ValueError):
    """A diagnostic containing a location and fixed cause, never source values."""

    def __init__(self, line: int, cause: str = "syntax"):
        self.line, self.cause = line, cause
        super().__init__(f"dotReg line {line}: {cause}")


@dataclass(slots=True)
class Entry:
    line: int
    kind: str
    path: list[str]
    raw: str = ""
    unset: bool = False
    type: str = ""
    has_type: bool = False
    form: str = ""

    def as_dict(self) -> dict:
        result = {"line": self.line, "kind": self.kind, "path": self.path,
                  "raw": self.raw, "unset": self.unset, "has_type": self.has_type}
        if self.type:
            result["type"] = self.type
        if self.form:
            result["form"] = self.form
        return result


@dataclass(frozen=True, slots=True)
class _Node:
    parent: _Node | None
    suffix: tuple[str, ...]
    count: int

    def elements(self, suffix: list[str] | None = None) -> list[str]:
        path = [""] * (self.count + (len(suffix) if suffix else 0))
        if suffix:
            path[self.count:] = suffix
        end, node = self.count, self
        while node is not None:
            start = end - len(node.suffix)
            path[start:end] = node.suffix
            end, node = start, node.parent
        return path


@dataclass(slots=True)
class _Line:
    data: bytes
    ending: str
    number: int
    eof: bool = False
    failed: bool = False

    def validate(self) -> str:
        if self.failed:
            raise ParseError(self.number, "input")
        try:
            text = self.data.decode("utf-8", errors="strict")
        except UnicodeDecodeError:
            raise ParseError(self.number) from None
        if "\r" in text:
            raise ParseError(self.number)
        return text


class _Lines:
    def __init__(self, source: BinaryIO):
        self.source, self.number = source, 1
        self.buffer, self.offset, self.eof = b"", 0, False
        self.pending: _Line | None = None

    def next(self) -> _Line:
        if self.pending is not None:
            line, self.pending = self.pending, None
            return line
        pieces: list[bytes] = []
        failed = False
        while True:
            end = self.buffer.find(b"\n", self.offset)
            if end >= 0:
                pieces.append(self.buffer[self.offset:end + 1])
                self.offset = end + 1
                break
            pieces.append(self.buffer[self.offset:])
            self.buffer, self.offset = b"", 0
            if self.eof:
                break
            try:
                chunk = self.source.read(8192)
                if not isinstance(chunk, bytes):
                    raise TypeError()
            except Exception:
                failed = True
                break
            if not chunk:
                self.eof = True
                break
            self.buffer = chunk
        data, ending = b"".join(pieces), ""
        if data.endswith(b"\n"):
            data, ending = data[:-1], "\n"
            if data.endswith(b"\r"):
                data, ending = data[:-1], "\r\n"
        if self.number == 1 and data.startswith(b"\xef\xbb\xbf"):
            data = data[3:]
        line = _Line(data, ending, self.number, self.eof and not ending, failed)
        self.number += 1
        return line

    def properties(self) -> bool:
        declared = False
        while True:
            line = self.next()
            text = line.validate().rstrip(" \t")
            if not text:
                if not declared or (not line.data and not line.ending):
                    raise ParseError(line.number)
                return line.eof
            split = next((i for i, c in enumerate(text) if c in " \t"), -1)
            if split <= 0 or text[0] != "@":
                raise ParseError(line.number)
            name, value = text[:split], text[split:].lstrip(" \t")
            if not value:
                raise ParseError(line.number)
            if name != "@dotReg":
                raise ParseError(line.number, "unsupported property")
            if declared:
                raise ParseError(line.number, "duplicate property")
            if value != "0.1":
                raise ParseError(line.number, "unsupported version")
            declared = True
            if line.eof:
                raise ParseError(line.number + 1)

    def body(self, entry: Entry, indent: int, eof: bool) -> bool:
        if entry.raw == '"""':
            body: list[tuple[str, str]] = []
            while True:
                line = self.next()
                text = line.validate()
                margin = _spaces(text)
                if text[margin:].rstrip(" \t") == '"""':
                    parts: list[str] = []
                    for i, (part, ending) in enumerate(body):
                        if part:
                            if _spaces(part) < margin:
                                raise ParseError(entry.line + i + 1)
                            part = part[margin:]
                        if i:
                            parts.append(body[i - 1][1])
                        parts.append(part)
                    entry.raw, entry.form, entry.unset = "".join(parts), "triple_quoted", False
                    return line.eof
                if line.eof:
                    raise ParseError(entry.line)
                body.append((text, line.ending))
        if entry.unset and not eof:
            parts: list[str] = []
            found, previous, block_eof = False, "", False
            while True:
                line = self.next()
                prefix = len(line.data) - len(line.data.lstrip(b" "))
                if prefix <= indent or line.data[prefix:prefix + 1] != b"|":
                    self.pending = line
                    break
                text = line.validate()
                if found:
                    parts.append(previous)
                parts.append(text[prefix + 1:])
                found, previous = True, line.ending
                if line.eof:
                    block_eof = True
                    break
            if found:
                entry.raw, entry.form, entry.unset = "".join(parts), "pipe", False
                return block_eof
        return eof


def _spaces(text: str) -> int:
    return len(text) - len(text.lstrip(" "))


def _white(c: str) -> bool:
    # Unicode White_Space, matching the format rather than Python's extra C0 set.
    n = ord(c)
    return (9 <= n <= 13 or n in (32, 133, 160, 5760, 8232, 8233, 8239, 8287, 12288)
            or 8192 <= n <= 8202)


def _escape(text: str, start: int) -> tuple[str, int]:
    if start + 1 >= len(text):
        raise ValueError()
    end, c = start + 2, text[start + 1]
    basic = {'.': '.', '"': '"', '\\': '\\', '/': '/', 'b': '\b',
             'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}
    if c in basic:
        return basic[c], end
    if c != "u" or end >= len(text):
        raise ValueError()

    def hex_value(digits: str) -> int:
        if not digits or any(ch not in "0123456789abcdefABCDEF" for ch in digits):
            raise ValueError()
        return int(digits, 16)

    if text[end] == "{":
        close = text.find("}", end + 1, end + 8)
        if close < 0:
            raise ValueError()
        digits = text[end + 1:close]
        if not 1 <= len(digits) <= 6:
            raise ValueError()
        value, end = hex_value(digits), close + 1
    else:
        if end + 4 > len(text):
            raise ValueError()
        value, end = hex_value(text[end:end + 4]), end + 4
        if 0xD800 <= value <= 0xDBFF:
            if text[end:end + 2] != "\\u" or end + 6 > len(text):
                raise ValueError()
            low = hex_value(text[end + 2:end + 6])
            if not 0xDC00 <= low <= 0xDFFF:
                raise ValueError()
            value, end = 0x10000 + (value - 0xD800) * 1024 + low - 0xDC00, end + 6
    if value > 0x10FFFF or 0xD800 <= value <= 0xDFFF:
        raise ValueError()
    return chr(value), end


def _quoted(text: str, start: int = 0, decode: bool = True) -> tuple[str, int]:
    if start >= len(text) or text[start] != '"':
        raise ValueError()
    parts: list[str] = []
    i, literal = start + 1, start + 1
    while i < len(text):
        c = text[i]
        if c == '"':
            if decode:
                parts.append(text[literal:i])
            return "".join(parts), i + 1
        if c in "\r\n":
            raise ValueError()
        if c == "\\":
            value, end = _escape(text, i)
            if decode:
                parts.extend((text[literal:i], value))
            i, literal = end, end
        else:
            i += 1
    raise ValueError()


def _split(body: str, line: int) -> tuple[Entry, str]:
    entry, i = Entry(line, "", []), 0
    while i < len(body):
        c = body[i]
        if c == '"':
            try:
                _, i = _quoted(body, i, False)
            except ValueError:
                raise ParseError(line) from None
            continue
        if c == ":":
            key, rhs = body[:i].strip(" \t"), body[i + 1:]
        elif c == "(":
            key = body[:i].rstrip(" ")
            if key.endswith("\t"):
                raise ParseError(line, "unsupported type syntax")
            depth, close = 1, i + 1
            while close < len(body):
                t = body[close]
                if _white(t):
                    raise ParseError(line)
                if t in '\\"':
                    raise ParseError(line, "unsupported type syntax")
                depth += (t == "(") - (t == ")")
                if depth == 0:
                    break
                close += 1
            if depth:
                raise ParseError(line)
            entry.type = body[i + 1:close]
            if not entry.type:
                raise ParseError(line, "unsupported type syntax")
            tail = body[close + 1:].lstrip(" ")
            if tail.startswith("\t"):
                raise ParseError(line, "unsupported type syntax")
            if not tail.startswith(":"):
                raise ParseError(line)
            entry.has_type, rhs = True, tail[1:]
        else:
            i += 1
            continue
        entry.kind, entry.raw = "value", rhs.strip(" \t")
        entry.unset = not entry.raw
        return entry, key
    if body.endswith("."):
        entry.kind = "heading"
        return entry, body[:-1]
    raise ParseError(line)


def _path(key: str, line: int) -> list[str]:
    parts: list[str] = []
    i = 0
    while True:
        if i >= len(key):
            raise ParseError(line)
        if key[i] == '"':
            try:
                element, end = _quoted(key, i)
            except ValueError:
                raise ParseError(line) from None
            if end < len(key) and key[end] != ".":
                raise ParseError(line)
        else:
            end = key.find(".", i)
            if end < 0:
                end = len(key)
            element = key[i:end]
            if not element or any(_white(c) or unicodedata.category(c) == "Cc"
                                  or c in ':()#"\\' for c in element):
                raise ParseError(line)
        parts.append(element)
        if end == len(key):
            return parts
        i = end + 1


def parse_each(source: BinaryIO, visitor: Callable[[Entry], object]) -> None:
    """Visit complete entries; False stops, raised visitor errors pass unchanged.

    Source must provide read(size) returning bytes. Neither source nor visitor
    is closed. One boundary line and byte read-ahead may be buffered. Paths
    delivered to visitors are detached from the persistent heading nodes.
    """
    if source is None or not callable(getattr(source, "read", None)) or not callable(visitor):
        raise ParseError(0, "input")
    lines = _Lines(source)
    if lines.properties():
        return
    scopes: list[_Node] = []
    while True:
        physical = lines.next()
        source_line, eof = physical.validate(), physical.eof
        trimmed = source_line.strip(" \t")
        if trimmed and not trimmed.startswith("#"):
            indent = _spaces(source_line)
            body = source_line[indent:].rstrip(" \t")
            relative = body.startswith(".")
            if body.startswith("\t") or indent % 2 or relative != (indent > 0):
                raise ParseError(physical.number)
            entry, key = _split(body, physical.number)
            parent = None
            if relative:
                depth = indent // 2
                if depth > len(scopes):
                    raise ParseError(physical.number)
                del scopes[depth:]
                parent, key = scopes[depth - 1], key[1:]
            else:
                scopes.clear()
            if entry.kind == "heading":
                suffix = tuple(_path(key, physical.number))
                node = _Node(parent, suffix, len(suffix) + (parent.count if parent else 0))
                entry.path = node.elements()
                scopes.append(node)
            else:
                suffix = None if relative and not key else _path(key, physical.number)
                entry.path = parent.elements(suffix) if parent else suffix
                eof = lines.body(entry, indent, eof)
            if visitor(entry) is False:
                return
        if eof:
            return


def parse(source: BinaryIO) -> list[Entry]:
    """Collect entries or raise an error without returning partial results."""
    entries: list[Entry] = []
    parse_each(source, entries.append)
    return entries


def decode_string(entry: Entry) -> str:
    """Decode a value without interpreting its opaque type annotation."""
    def fail() -> None:
        raise ParseError(entry.line, "string decoding")

    if entry.kind != "value" or any(0xD800 <= ord(c) <= 0xDFFF for c in entry.raw):
        fail()
    if entry.form:
        if entry.unset or any(c == "\r" and entry.raw[i + 1:i + 2] != "\n"
                              for i, c in enumerate(entry.raw)):
            fail()
        if entry.form == "pipe":
            return entry.raw
        if entry.form != "triple_quoted":
            fail()
        parts, i, literal = [], 0, 0
        try:
            while i < len(entry.raw):
                if entry.raw[i] == "\\":
                    value, end = _escape(entry.raw, i)
                    parts.extend((entry.raw[literal:i], value))
                    literal = i = end
                else:
                    i += 1
        except ValueError:
            fail()
        parts.append(entry.raw[literal:])
        return "".join(parts)
    raw = entry.raw.strip(" \t")
    if "\r" in raw or "\n" in raw or entry.unset != (not raw):
        fail()
    if not raw or not raw.startswith('"'):
        return raw
    try:
        value, end = _quoted(raw)
    except ValueError:
        fail()
    if end != len(raw):
        fail()
    return value


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--mode", choices=("raw", "strings"), default="strings")
    parser.add_argument("file", nargs="?")
    args = parser.parse_args()
    try:
        if args.file:
            with open(args.file, "rb") as source:
                entries = parse(source)
        else:
            entries = parse(sys.stdin.buffer)
        result = []
        for entry in entries:
            item = entry.as_dict()
            if args.mode == "strings" and entry.kind == "value":
                item["decoded"] = decode_string(entry)
            result.append(item)
        output = json.dumps(result, ensure_ascii=True, separators=(",", ":"))
    except ParseError as error:
        print(error, file=sys.stderr)
        return 1
    except Exception:
        print("dotReg: input or output failure", file=sys.stderr)
        return 1
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
