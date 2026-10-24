import io
import unittest

import dotreg


def document(body: str | bytes) -> io.BytesIO:
    return io.BytesIO(b"@dotReg 1\n\n" + (body.encode() if isinstance(body, str) else body))


class ParserTests(unittest.TestCase):
    def test_entries_scope_boundaries_and_repetition(self):
        entries = dotreg.parse(document('A.B.\n  ."C.D"(list(int)): 001\n  .: x\n  .: x\n"":\n'))
        self.assertEqual([e.path for e in entries], [
            ["A", "B"], ["A", "B", "C.D"], ["A", "B"], ["A", "B"], [""]])
        self.assertEqual(entries[1].type, "list(int)")
        self.assertTrue(entries[1].has_type)
        self.assertTrue(entries[-1].unset)
        self.assertEqual(entries[1].line, 4)

    def test_mutation_does_not_change_scopes(self):
        visited = []

        def visitor(entry):
            visited.append(entry.path[:])
            entry.path[:] = ["mutated"]

        dotreg.parse_each(document("A.\n  .B.\n    .C:x\n  .D:y"), visitor)
        self.assertEqual(visited, [["A"], ["A", "B"], ["A", "B", "C"], ["A", "D"]])

    def test_direct_value_paths_are_independent_across_scope_changes(self):
        retained, visited = [], []

        def visitor(entry):
            retained.append(entry.path)
            visited.append(entry.path[:])
            entry.path[:] = ["changed"]

        dotreg.parse_each(document(
            'Root.Zero:zero\nA.B.\n  .:one\n  .C.D:two\n'
            '  ."C.D"."":three\n  .E.\n    .:four\n    .F:five\n'
            '  .G:six\nZ:seven\n"":eight\n'), visitor)
        self.assertEqual(visited, [
            ["Root", "Zero"], ["A", "B"], ["A", "B"], ["A", "B", "C", "D"],
            ["A", "B", "C.D", ""], ["A", "B", "E"], ["A", "B", "E"],
            ["A", "B", "E", "F"], ["A", "B", "G"], ["Z"], [""]])
        self.assertEqual(len({id(path) for path in retained}), len(retained))
        self.assertEqual(retained, [["changed"]] * len(retained))

    def test_visitor_stop_does_not_validate_later_entry(self):
        calls = []
        dotreg.parse_each(document(b"A:good\n\xff:bad"), lambda entry: calls.append(entry) or False)
        self.assertEqual(len(calls), 1)

    def test_visitor_error_is_unchanged(self):
        sentinel = RuntimeError("recipient-owned error")

        def visitor(entry):
            raise sentinel

        with self.assertRaises(RuntimeError) as caught:
            dotreg.parse_each(document("A:first\ninvalid"), visitor)
        self.assertIs(caught.exception, sentinel)

    def test_pipe_lookahead_error_is_deferred(self):
        for body in (b"A:\n\xff", b"A:\n |body\n\xff", b"A:\nB:bad\r"):
            visited = []
            dotreg.parse_each(document(body), lambda entry: visited.append(entry) or False)
            self.assertEqual(len(visited), 1)
        with self.assertRaises(dotreg.ParseError):
            dotreg.parse(document(b"A:\n |body\n\xff"))

    def test_invalid_qualifying_pipe_body_precedes_notification(self):
        visited = []
        with self.assertRaises(dotreg.ParseError) as caught:
            dotreg.parse_each(document(b"A:\n |\xff"), lambda entry: visited.append(entry) or False)
        self.assertEqual(caught.exception.line, 4)
        self.assertEqual(visited, [])

    def test_multiline_and_original_line_endings(self):
        entries = dotreg.parse(document(b'A:"""\r\n  first\\nsecond\r\n    tail\n  """\r\nB:\n |one\r\n |\nC: last'))
        self.assertEqual(entries[0].raw, "first\\nsecond\r\n  tail")
        self.assertEqual(dotreg.decode_string(entries[0]), "first\nsecond\r\n  tail")
        self.assertEqual(entries[1].raw, "one\r\n")
        self.assertEqual(entries[1].form, "pipe")
        self.assertFalse(entries[1].unset)
        self.assertEqual(entries[2].path, ["C"])

    def test_unicode_and_literal_rules(self):
        entries = dotreg.parse(document('"\\u{1F600}"."\\u304C": "\\uD83D\\uDE00\\.\\t"\n'
                                        'A: literal"\\q\nB: ""\nC:\n'))
        self.assertEqual(entries[0].path, ["😀", "が"])
        self.assertEqual(dotreg.decode_string(entries[0]), "😀.\t")
        self.assertEqual(dotreg.decode_string(entries[1]), 'literal"\\q')
        self.assertFalse(entries[2].unset)
        self.assertTrue(entries[3].unset)

    def test_raw_accepts_bad_value_escape_but_decoding_fails(self):
        entry = dotreg.parse(document('A: "\\q"'))[0]
        with self.assertRaises(dotreg.ParseError):
            dotreg.decode_string(entry)

    def test_required_properties_encoding_and_margin(self):
        invalid = [b"", b"@dotReg 1\n", b"\n", b"@dotReg 1\n@dotReg 1\n\n",
                   b"@dotReg 2\n\n", b"@other 1\n\n", b"@dotReg 1\n\n#\xff",
                   b'@dotReg 1\n\nA:"""\n x\n  """', b"@dotReg 1\n\nA:x\r"]
        for data in invalid:
            with self.subTest(data=data), self.assertRaises(dotreg.ParseError):
                dotreg.parse(io.BytesIO(data))
        self.assertEqual(dotreg.parse(io.BytesIO(b"\xef\xbb\xbf@dotReg 1\n\n")), [])

    def test_input_error_is_safe_and_sources_are_required(self):
        class Failing:
            def read(self, size):
                raise OSError("SECRET INPUT VALUE")

        for source, visitor in ((Failing(), lambda e: None), (None, lambda e: None),
                                (io.BytesIO(), None), (io.StringIO(""), lambda e: None)):
            with self.assertRaises(dotreg.ParseError) as caught:
                dotreg.parse_each(source, visitor)
            self.assertNotIn("SECRET", str(caught.exception))

    def test_long_lines_and_fresh_state(self):
        value = "x" * 200_000
        self.assertEqual(dotreg.parse(document("A:" + value))[0].raw, value)
        dotreg.parse(document("A."))
        with self.assertRaises(dotreg.ParseError):
            dotreg.parse(document("  .B:x"))

    def test_decode_rejects_inconsistent_entries(self):
        for entry in (dotreg.Entry(3, "heading", ["A"]),
                      dotreg.Entry(3, "value", ["A"], "x", True),
                      dotreg.Entry(3, "value", ["A"], "a\rb", False, form="pipe"),
                      dotreg.Entry(3, "value", ["A"], "\ud800")):
            with self.assertRaises(dotreg.ParseError):
                dotreg.decode_string(entry)


if __name__ == "__main__":
    unittest.main()
