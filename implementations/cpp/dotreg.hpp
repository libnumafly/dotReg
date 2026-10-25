#ifndef DOTREG_SAMPLE_HPP
#define DOTREG_SAMPLE_HPP

#include <cstdint>
#include <functional>
#include <istream>
#include <memory>
#include <optional>
#include <stdexcept>
#include <string>
#include <string_view>
#include <utility>
#include <vector>

namespace dotreg {
struct Entry {
    std::size_t line = 0;
    std::string kind, raw;
    std::vector<std::string> path;
    bool unset = false, has_type = false;
    std::string type_expression, form;
};
struct ParseError : std::runtime_error {
    std::size_t line;
    explicit ParseError(std::size_t position, const char* cause = "invalid syntax")
        : std::runtime_error("dotReg line " + std::to_string(position) + ": " + cause), line(position) {}
};

namespace detail {
inline std::string_view trim(std::string_view s) {
    auto begin = s.find_first_not_of(" \t");
    if (begin == s.npos) return {};
    return s.substr(begin, s.find_last_not_of(" \t") - begin + 1);
}
inline std::string_view right_trim(std::string_view s, std::string_view chars = " \t") {
    auto end = s.find_last_not_of(chars);
    return end == s.npos ? std::string_view{} : s.substr(0, end + 1);
}
inline std::string_view left_trim(std::string_view s, std::string_view chars = " \t") {
    auto start = s.find_first_not_of(chars);
    return start == s.npos ? std::string_view{} : s.substr(start);
}
inline std::size_t spaces(std::string_view s) {
    auto count = s.find_first_not_of(' '); return count == s.npos ? s.size() : count;
}
inline bool whitespace(std::uint32_t c) {
    return (c >= 9 && c <= 13) || c == 32 || c == 0x85 || c == 0xa0 || c == 0x1680 ||
        (c >= 0x2000 && c <= 0x200a) || c == 0x2028 || c == 0x2029 || c == 0x202f || c == 0x205f || c == 0x3000;
}
// Strict Unicode scalar decoding: rejects overlong forms, surrogates and truncation.
inline std::uint32_t scalar(std::string_view s, std::size_t& i, std::size_t line) {
    auto first = static_cast<unsigned char>(s[i++]);
    if (first < 0x80) return first;
    std::uint32_t value; unsigned count; std::uint32_t minimum;
    if (first >= 0xc2 && first <= 0xdf) { value = first & 31; count = 1; minimum = 0x80; }
    else if (first >= 0xe0 && first <= 0xef) { value = first & 15; count = 2; minimum = 0x800; }
    else if (first >= 0xf0 && first <= 0xf4) { value = first & 7; count = 3; minimum = 0x10000; }
    else throw ParseError(line);
    if (s.size() - i < count) throw ParseError(line);
    while (count--) {
        auto c = static_cast<unsigned char>(s[i++]);
        if ((c & 0xc0) != 0x80) throw ParseError(line);
        value = value * 64 + (c & 63);
    }
    if (value < minimum || value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff)) throw ParseError(line);
    return value;
}
inline void validate(std::string_view s, std::size_t line, bool block = false) {
    for (std::size_t i = 0; i < s.size();) {
        if (s[i] == '\r' && (!block || i + 1 == s.size() || s[i + 1] != '\n')) throw ParseError(line);
        scalar(s, i, line);
    }
}
inline void append_scalar(std::string& out, std::uint32_t c, std::size_t line) {
    if (c > 0x10ffff || (c >= 0xd800 && c <= 0xdfff)) throw ParseError(line, "invalid string");
    if (c < 0x80) out += static_cast<char>(c);
    else if (c < 0x800) { out += static_cast<char>(0xc0 | c >> 6); out += static_cast<char>(0x80 | (c & 63)); }
    else if (c < 0x10000) { out += static_cast<char>(0xe0 | c >> 12); out += static_cast<char>(0x80 | ((c >> 6) & 63)); out += static_cast<char>(0x80 | (c & 63)); }
    else { out += static_cast<char>(0xf0 | c >> 18); out += static_cast<char>(0x80 | ((c >> 12) & 63)); out += static_cast<char>(0x80 | ((c >> 6) & 63)); out += static_cast<char>(0x80 | (c & 63)); }
}
inline unsigned hex(char c, std::size_t line) {
    if (c >= '0' && c <= '9') return c - '0';
    if (c >= 'a' && c <= 'f') return c - 'a' + 10;
    if (c >= 'A' && c <= 'F') return c - 'A' + 10;
    throw ParseError(line, "invalid string");
}
inline std::uint32_t four(std::string_view s, std::size_t start, std::size_t line) {
    if (start > s.size() || s.size() - start < 4) throw ParseError(line, "invalid string");
    std::uint32_t value = 0;
    for (auto i = start; i < start + 4; ++i) value = value * 16 + hex(s[i], line);
    return value;
}
inline std::uint32_t escape(std::string_view s, std::size_t& i, std::size_t line) {
    if (++i >= s.size()) throw ParseError(line, "invalid string");
    auto c = s[i++];
    switch (c) {
    case '.': case '"': case '\\': case '/': return c;
    case 'b': return 8; case 'f': return 12; case 'n': return 10; case 'r': return 13; case 't': return 9;
    case 'u': break;
    default: throw ParseError(line, "invalid string");
    }
    if (i < s.size() && s[i] == '{') {
        ++i; unsigned digits = 0; std::uint32_t value = 0;
        while (i < s.size() && s[i] != '}') {
            if (++digits > 6) throw ParseError(line, "invalid string");
            value = value * 16 + hex(s[i++], line);
        }
        if (!digits || i == s.size() || value > 0x10ffff || (value >= 0xd800 && value <= 0xdfff)) throw ParseError(line, "invalid string");
        ++i; return value;
    }
    auto value = four(s, i, line); i += 4;
    if (value >= 0xd800 && value <= 0xdbff) {
        if (s.size() - i < 6 || s.substr(i, 2) != "\\u") throw ParseError(line, "invalid string");
        auto low = four(s, i + 2, line);
        if (low < 0xdc00 || low > 0xdfff) throw ParseError(line, "invalid string");
        value = 0x10000 + (value - 0xd800) * 0x400 + low - 0xdc00; i += 6;
    }
    if (value >= 0xd800 && value <= 0xdfff) throw ParseError(line, "invalid string");
    return value;
}
inline std::size_t quoted(std::string_view s, std::size_t start, std::size_t line, std::string* output = nullptr) {
    if (start >= s.size() || s[start] != '"') throw ParseError(line, "invalid string");
    std::size_t i = start + 1, literal = i;
    while (i < s.size()) {
        if (s[i] == '"') { if (output) output->append(s.substr(literal, i - literal)); return i + 1; }
        if (s[i] == '\r' || s[i] == '\n') throw ParseError(line, "invalid string");
        if (s[i] == '\\') {
            auto begin = i; auto c = escape(s, i, line);
            if (output) { output->append(s.substr(literal, begin - literal)); append_scalar(*output, c, line); }
            literal = i;
        } else ++i;
    }
    throw ParseError(line, "invalid string");
}
struct Physical {
    std::string text, ending; std::size_t number = 0; bool eof = false, failed = false;
    void validate() const { if (failed) throw ParseError(number, "input failure"); detail::validate(text, number); }
};
class Lines {
    std::istream& input; std::size_t number = 1; std::optional<Physical> pending;
public:
    explicit Lines(std::istream& source) : input(source) {}
    Physical next() {
        if (pending) { auto line = std::move(*pending); pending.reset(); return line; }
        Physical line; line.number = number++;
        try { std::getline(input, line.text); }
        catch (...) { line.failed = !input.eof() || input.bad(); }
        line.eof = input.eof();
        line.failed = line.failed || input.bad() || (input.fail() && !input.eof());
        if (!line.eof && !line.failed) {
            line.ending = "\n";
            if (!line.text.empty() && line.text.back() == '\r') { line.text.pop_back(); line.ending = "\r\n"; }
        }
        if (line.number == 1 && line.text.compare(0, 3, "\xef\xbb\xbf") == 0) line.text.erase(0, 3);
        return line;
    }
    bool properties() {
        bool declared = false;
        for (;;) {
            auto line = next(); line.validate(); auto text = right_trim(line.text);
            if (text.empty()) {
                if (!declared || (line.text.empty() && line.ending.empty())) throw ParseError(line.number);
                return line.eof;
            }
            auto sep = text.find_first_of(" \t");
            if (sep == text.npos || sep == 0 || text[0] != '@') throw ParseError(line.number);
            if (declared || text.substr(0, sep) != "@dotReg" || left_trim(text.substr(sep)) != "0.1") throw ParseError(line.number);
            declared = true;
            if (line.eof) throw ParseError(line.number + 1);
        }
    }
    bool value_body(Entry& e, std::size_t indent, bool eof) {
        if (e.raw == "\"\"\"") {
            std::vector<Physical> body;
            for (;;) {
                auto line = next(); line.validate(); auto margin = spaces(line.text);
                if (right_trim(std::string_view(line.text).substr(margin)) == "\"\"\"") {
                    std::string raw;
                    for (std::size_t i = 0; i < body.size(); ++i) {
                        if (i) raw += body[i - 1].ending;
                        if (!body[i].text.empty()) {
                            if (spaces(body[i].text) < margin) throw ParseError(body[i].number);
                            raw.append(body[i].text, margin, std::string::npos);
                        }
                    }
                    e.raw = std::move(raw); e.form = "triple_quoted"; e.unset = false; return line.eof;
                }
                if (line.eof) throw ParseError(e.line);
                body.push_back(std::move(line));
            }
        }
        if (e.unset && !eof) {
            std::string raw, ending; bool found = false;
            for (;;) {
                auto line = next(); auto prefix = spaces(line.text);
                if (prefix <= indent || prefix == line.text.size() || line.text[prefix] != '|') {
                    pending = std::move(line);
                    if (found) { e.raw = std::move(raw); e.form = "pipe"; e.unset = false; }
                    return false;
                }
                line.validate(); if (found) raw += ending;
                raw.append(line.text, prefix + 1, std::string::npos); found = true; ending = line.ending;
                if (line.eof) { e.raw = std::move(raw); e.form = "pipe"; e.unset = false; return true; }
            }
        }
        return eof;
    }
};
inline std::pair<Entry, std::string_view> split_line(std::string_view body, std::size_t line) {
    Entry e; e.line = line; e.kind = "heading";
    for (std::size_t i = 0; i < body.size();) {
        if (body[i] == '"') { i = quoted(body, i, line); continue; }
        std::string_view key, rhs;
        if (body[i] == ':') { key = trim(body.substr(0, i)); rhs = body.substr(i + 1); }
        else if (body[i] == '(') {
            key = right_trim(body.substr(0, i), " ");
            if (!key.empty() && key.back() == '\t') throw ParseError(line);
            std::size_t depth = 1, close = body.npos;
            for (std::size_t j = i + 1; j < body.size();) {
                auto begin = j; auto c = scalar(body, j, line);
                if (whitespace(c) || c == '"' || c == '\\') throw ParseError(line);
                if (c == '(') ++depth;
                if (c == ')' && --depth == 0) { close = begin; break; }
            }
            if (close == body.npos || close == i + 1) throw ParseError(line);
            auto tail = left_trim(body.substr(close + 1), " ");
            if (tail.empty() || tail[0] != ':') throw ParseError(line);
            e.has_type = true; e.type_expression = body.substr(i + 1, close - i - 1); rhs = tail.substr(1);
        } else { ++i; continue; }
        e.kind = "value"; e.raw = trim(rhs); e.unset = e.raw.empty(); return {std::move(e), key};
    }
    if (!body.empty() && body.back() == '.') return {std::move(e), body.substr(0, body.size() - 1)};
    throw ParseError(line);
}
inline void append_path(std::string_view key, std::size_t line, std::vector<std::string>& result) {
    for (;;) {
        if (key.empty()) throw ParseError(line);
        std::string element;
        if (key[0] == '"') {
            auto end = quoted(key, 0, line, &element); key.remove_prefix(end);
            if (!key.empty() && key[0] != '.') throw ParseError(line);
        } else {
            auto end = key.find('.'); if (end == key.npos) end = key.size();
            auto raw = key.substr(0, end);
            if (raw.empty()) throw ParseError(line);
            for (std::size_t i = 0; i < raw.size();) {
                auto c = scalar(raw, i, line);
                if (whitespace(c) || c <= 31 || (c >= 127 && c <= 159) || (c < 128 && std::string_view(":()#\"\\").find(static_cast<char>(c)) != std::string_view::npos)) throw ParseError(line);
            }
            element = raw; key.remove_prefix(end);
        }
        result.push_back(std::move(element)); if (key.empty()) return; key.remove_prefix(1);
    }
}
inline std::vector<std::string> path(std::string_view key, std::size_t line) {
    std::vector<std::string> result; append_path(key, line, result); return result;
}
struct Node {
    std::shared_ptr<const Node> parent; std::vector<std::string> suffix; std::size_t count;
    std::vector<std::string> elements(std::size_t extra = 0) const {
        std::vector<const Node*> parts;
        for (auto node = this; node; node = node->parent.get()) parts.push_back(node);
        std::vector<std::string> result; result.reserve(count + extra);
        for (auto it = parts.rbegin(); it != parts.rend(); ++it) result.insert(result.end(), (*it)->suffix.begin(), (*it)->suffix.end());
        return result;
    }
};
struct Scopes {
    std::vector<std::shared_ptr<const Node>> nodes;
    void truncate(std::size_t n) { while (nodes.size() > n) nodes.pop_back(); }
    ~Scopes() { truncate(0); }
};
} // namespace detail

using Visitor = std::function<void(Entry)>;
// Visitor exceptions propagate unchanged. Lookahead is validated only on its turn.
inline void parse_each(std::istream* input, const Visitor& visit) {
    if (!input || !visit) throw ParseError(0, "input or visitor required");
    detail::Lines lines(*input); if (lines.properties()) return;
    detail::Scopes scopes;
    for (;;) {
        auto physical = lines.next(); physical.validate(); bool eof = physical.eof;
        auto trimmed = detail::trim(physical.text);
        if (!trimmed.empty() && trimmed[0] != '#') {
            auto indent = detail::spaces(physical.text);
            auto body = detail::right_trim(std::string_view(physical.text).substr(indent));
            if (body[0] == '\t' || indent % 2) throw ParseError(physical.number);
            bool relative = body[0] == '.';
            if (relative != (indent > 0)) throw ParseError(physical.number);
            auto split = detail::split_line(body, physical.number); auto& e = split.first; auto key = split.second;
            std::shared_ptr<const detail::Node> parent;
            if (relative) {
                auto depth = indent / 2; if (depth > scopes.nodes.size()) throw ParseError(physical.number);
                scopes.truncate(depth); parent = scopes.nodes[depth - 1]; key.remove_prefix(1);
            } else scopes.truncate(0);
            if (e.kind == "heading") {
                auto suffix = detail::path(key, physical.number); auto count = suffix.size() + (parent ? parent->count : 0);
                auto node = std::make_shared<detail::Node>(detail::Node{parent, std::move(suffix), count});
                e.path = node->elements();
                scopes.nodes.push_back(std::move(node));
            } else {
                bool self_assignment = relative && key.empty();
                if (parent) e.path = parent->elements(self_assignment ? 0 : 1);
                if (!self_assignment) detail::append_path(key, physical.number, e.path);
                eof = lines.value_body(e, indent, eof);
            }
            visit(std::move(e));
        }
        if (eof) return;
    }
}
inline void parse_each(std::istream& input, const Visitor& visit) { parse_each(&input, visit); }
// The local collection is destroyed on failure; no partial result escapes.
inline std::vector<Entry> parse(std::istream& input) {
    std::vector<Entry> entries; parse_each(input, [&](Entry e) { entries.push_back(std::move(e)); }); return entries;
}
inline std::string decode_string(const Entry& e) {
    if (e.kind != "value") throw ParseError(e.line, "invalid string");
    if (!e.form.empty()) {
        if (e.unset) throw ParseError(e.line, "invalid string");
        detail::validate(e.raw, e.line, true);
        if (e.form == "pipe") return e.raw;
        if (e.form != "triple_quoted") throw ParseError(e.line, "invalid string");
        std::string result; std::size_t start = 0;
        for (;;) {
            auto slash = e.raw.find('\\', start);
            if (slash == e.raw.npos) { result.append(e.raw, start, std::string::npos); return result; }
            result.append(e.raw, start, slash - start); start = slash;
            auto c = detail::escape(e.raw, start, e.line); detail::append_scalar(result, c, e.line);
        }
    }
    auto raw = detail::trim(e.raw); detail::validate(raw, e.line);
    if (raw.find('\n') != raw.npos || e.unset != raw.empty()) throw ParseError(e.line, "invalid string");
    if (raw.empty() || raw[0] != '"') return std::string(raw);
    std::string result;
    if (detail::quoted(raw, 0, e.line, &result) != raw.size()) throw ParseError(e.line, "invalid string");
    return result;
}
inline void json_string(std::string_view s, std::string& out) {
    static constexpr char hex[] = "0123456789abcdef"; out += '"';
    for (unsigned char c : s) {
        if (c == '"') out += "\\\"";
        else if (c == '\\') out += "\\\\";
        else if (c == '\n') out += "\\n";
        else if (c == '\r') out += "\\r";
        else if (c == '\t') out += "\\t";
        else if (c < 32) { out += "\\u00"; out += hex[c >> 4]; out += hex[c & 15]; }
        else out += static_cast<char>(c);
    }
    out += '"';
}
} // namespace dotreg
#endif
