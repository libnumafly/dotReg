#include "dotreg.hpp"
#include <fstream>
#include <iostream>
#ifdef _WIN32
#include <fcntl.h>
#include <io.h>
#endif

int main(int argc, char** argv) {
#ifdef _WIN32
    _setmode(_fileno(stdin), _O_BINARY);
    _setmode(_fileno(stdout), _O_BINARY);
#endif
    try {
        std::string mode = "strings", file;
        for (int i = 1; i < argc; ++i) {
            std::string arg = argv[i];
            if (arg == "--mode" && i + 1 < argc) mode = argv[++i];
            else if ((!arg.empty() && arg[0] == '-') || !file.empty()) throw dotreg::ParseError(0, "invalid arguments");
            else file = arg;
        }
        if (mode != "raw" && mode != "strings") throw dotreg::ParseError(0, "invalid mode");
        std::ifstream source;
        if (!file.empty()) { source.open(file, std::ios::binary); if (!source) throw dotreg::ParseError(0, "input failure"); }
        std::istream& input = file.empty() ? std::cin : source;
        std::string output = "["; bool first = true;
        auto entries = dotreg::parse(input);
        for (const auto& e : entries) {
            std::optional<std::string> decoded;
            if (mode == "strings" && e.kind == "value") decoded = dotreg::decode_string(e);
            if (!first) output += ',';
            first = false;
            output += "{\"line\":" + std::to_string(e.line) + ",\"kind\":"; dotreg::json_string(e.kind, output);
            output += ",\"path\":[";
            for (std::size_t i = 0; i < e.path.size(); ++i) { if (i) output += ','; dotreg::json_string(e.path[i], output); }
            output += "],\"raw\":"; dotreg::json_string(e.raw, output);
            output += e.unset ? ",\"unset\":true" : ",\"unset\":false";
            output += e.has_type ? ",\"has_type\":true" : ",\"has_type\":false";
            if (!e.type_expression.empty()) { output += ",\"type\":"; dotreg::json_string(e.type_expression, output); }
            if (!e.form.empty()) { output += ",\"form\":"; dotreg::json_string(e.form, output); }
            if (decoded) { output += ",\"decoded\":"; dotreg::json_string(*decoded, output); }
            output += '}';
        }
        output += "]\n"; std::cout.write(output.data(), static_cast<std::streamsize>(output.size()));
        if (!std::cout) throw dotreg::ParseError(0, "output failure");
        return 0;
    } catch (const dotreg::ParseError& error) { std::cerr << error.what() << '\n'; }
    catch (...) { std::cerr << "dotReg line 0: input or resource failure\n"; }
    return 1;
}
