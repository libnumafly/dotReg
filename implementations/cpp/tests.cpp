#include "dotreg.hpp"
#include <iostream>
#include <sstream>

static void require(bool condition) { if (!condition) throw std::logic_error("test failed"); }
static void mutation_and_repetition() {
    std::istringstream source("@dotReg 1\n\nA.B.\n  .C:one\n  .C:one\n  .:root\n");
    std::vector<std::vector<std::string>> paths;
    dotreg::parse_each(source, [&](dotreg::Entry e) { paths.push_back(e.path); e.path[0] = "changed"; });
    require(paths == std::vector<std::vector<std::string>>{{"A", "B"}, {"A", "B", "C"}, {"A", "B", "C"}, {"A", "B"}});
}
static void mixed_value_paths() {
    std::istringstream source("@dotReg 1\n\nRoot.\"\".\n  .\"A\\.B\".\"\":one\n  .\"A\\.B\".\"\":two\n  .:self\n  .Child.\n    .\"leaf.name\":three\nOther.\"\":absolute\n");
    std::vector<std::vector<std::string>> paths;
    std::vector<dotreg::Entry> entries;
    dotreg::parse_each(source, [&](dotreg::Entry e) {
        paths.push_back(e.path); e.path[0] = "recipient mutation"; e.path.push_back("extra");
        entries.push_back(std::move(e));
    });
    require(paths == std::vector<std::vector<std::string>>{
        {"Root", ""}, {"Root", "", "A.B", ""}, {"Root", "", "A.B", ""},
        {"Root", ""}, {"Root", "", "Child"}, {"Root", "", "Child", "leaf.name"}, {"Other", ""}});
    require(entries[1].raw == "one" && entries[2].raw == "two" && entries[3].raw == "self" && entries[6].raw == "absolute");
    for (const auto& e : entries) require(e.path[0] == "recipient mutation" && e.path.back() == "extra");
}
static void visitor_identity_and_lookahead() {
    std::istringstream source("@dotReg 1\n\nA:\n\xff\n");
    int sentinel = 17, calls = 0; bool stopped = false;
    try { dotreg::parse_each(source, [&](dotreg::Entry) { ++calls; throw &sentinel; }); }
    catch (int* returned) { require(returned == &sentinel); stopped = true; }
    require(stopped && calls == 1);
    std::istringstream invalid_body("@dotReg 1\n\nA:\n |\xff\n"); calls = 0;
    try { dotreg::parse_each(invalid_body, [&](dotreg::Entry) { ++calls; }); require(false); }
    catch (const dotreg::ParseError& error) { require(error.line == 4 && calls == 0); }
}
class BrokenBuffer : public std::streambuf {
    std::string prefix;
protected:
    int_type underflow() override { throw std::runtime_error("PRIVATE SOURCE ERROR"); }
public:
    explicit BrokenBuffer(std::string text) : prefix(std::move(text)) { setg(prefix.data(), prefix.data(), prefix.data() + prefix.size()); }
};
static void source_errors_and_required_arguments() {
    BrokenBuffer buffer("@dotReg 1\n\nA:ok\n"); std::istream source(&buffer); bool failed = false;
    try { auto entries = dotreg::parse(source); (void)entries; }
    catch (const dotreg::ParseError& error) { require(error.line == 4); require(std::string(error.what()).find("PRIVATE") == std::string::npos); failed = true; }
    require(failed);
    BrokenBuffer later("@dotReg 1\n\nA:ok\n"); std::istream stop_source(&later);
    try { dotreg::parse_each(stop_source, [](dotreg::Entry) { throw 31; }); }
    catch (int result) { require(result == 31); }
    try { dotreg::parse_each(nullptr, [](dotreg::Entry) {}); require(false); }
    catch (const dotreg::ParseError& error) { require(error.line == 0); }
    std::istringstream empty;
    try { dotreg::parse_each(empty, {}); require(false); }
    catch (const dotreg::ParseError& error) { require(error.line == 0); }
}
static void multiline() {
    std::istringstream source("@dotReg 1\n\nA: \"\"\"\n   first\\n\r\n     second\n   \"\"\"\nB:\n |C:\\temp\\q\r\n |\n");
    auto entries = dotreg::parse(source);
    require(entries[0].raw == "first\\n\r\n  second");
    require(dotreg::decode_string(entries[0]) == "first\n\r\n  second");
    require(entries[1].form == "pipe" && dotreg::decode_string(entries[1]) == "C:\\temp\\q\r\n");
}
static void unicode_and_decoding() {
    std::istringstream source("@dotReg 1\n\n\"A\\.B\".\"\"(map(str,list(int))):\"\\uD83D\\uDE00\\u{0}\"\nA:\"\\q\"\n");
    auto entries = dotreg::parse(source);
    require(entries[0].path == std::vector<std::string>{"A.B", ""});
    require(entries[0].type_expression == "map(str,list(int))");
    require(dotreg::decode_string(entries[0]) == std::string("\xf0\x9f\x98\x80\0", 5));
    try { dotreg::decode_string(entries[1]); require(false); } catch (const dotreg::ParseError&) {}
}
static void invalid_sources() {
    for (std::string body : {"", "@dotReg 1\n", "@dotReg 1\n\nA:\xff\n", "@dotReg 1\n\nA:ok\r", "@dotReg 1\n\nA.\n  .B.\n    .C:yes\n  .D:yes\n    .E:no\n"}) {
        std::istringstream source(body); bool failed = false;
        try { auto entries = dotreg::parse(source); (void)entries; } catch (const dotreg::ParseError&) { failed = true; }
        require(failed);
    }
    std::istringstream empty_body("@dotReg 1\n\n"); require(dotreg::parse(empty_body).empty());
}
static void deep_paths() {
    std::string text = "@dotReg 1\n\nRoot.\n";
    for (unsigned depth = 1; depth < 1500; ++depth) { text.append(depth * 2, ' '); text += ".Child.\n"; }
    std::istringstream source(text); std::size_t count = 0;
    dotreg::parse_each(source, [&](dotreg::Entry e) { require(e.path.size() == ++count); }); require(count == 1500);
}
int main() {
    mutation_and_repetition(); mixed_value_paths(); visitor_identity_and_lookahead(); source_errors_and_required_arguments();
    multiline(); unicode_and_decoding(); invalid_sources(); deep_paths();
    std::cout << "8 C++ API tests passed\n";
}
