using System;
using System.Collections.Generic;
using System.Globalization;
using System.Text;

namespace ThinhThan.Core.Assets.Editor.AssetProduction
{
    /// <summary>
    /// Minimal deterministic JSON reader for the asset source register
    /// (presentation_asset_manifest.md section 6). No third-party JSON
    /// dependency: the register is parsed into a plain node tree and every
    /// ProvenanceValidator check reads typed accessors off the nodes.
    /// Duplicate keys inside an object are rejected; input must be UTF-8
    /// JSON with no trailing garbage.
    /// </summary>
    public static class RegisterJson
    {
        public sealed class Node
        {
            public enum Kind
            {
                Obj = 0,
                Arr = 1,
                Str = 2,
                Num = 3,
                Bool = 4,
                Null = 5,
            }

            public Kind Type;
            public Dictionary<string, Node>? Obj;
            public List<Node>? Arr;
            public string Str = string.Empty;
            public double Num;
            public bool Bool;

            public static Node ObjNode(Dictionary<string, Node> v)
            {
                return new Node { Type = Kind.Obj, Obj = v };
            }

            public static Node ArrNode(List<Node> v)
            {
                return new Node { Type = Kind.Arr, Arr = v };
            }

            public static Node StrNode(string v)
            {
                return new Node { Type = Kind.Str, Str = v };
            }

            public static Node NumNode(double v)
            {
                return new Node { Type = Kind.Num, Num = v };
            }

            public static Node BoolNode(bool v)
            {
                return new Node { Type = Kind.Bool, Bool = v };
            }

            public static Node NullNode()
            {
                return new Node { Type = Kind.Null };
            }

            public Node? Get(string key)
            {
                if (Type != Kind.Obj || Obj == null || !Obj.TryGetValue(key, out var n))
                {
                    return null;
                }
                return n;
            }

            public string? StringOrNull()
            {
                if (Type == Kind.Null)
                {
                    return null;
                }
                return Type == Kind.Str ? Str : null;
            }
        }

        public sealed class ParseException : Exception
        {
            public ParseException(string message, int offset) : base(
                string.Format(CultureInfo.InvariantCulture, "{0} (offset {1})", message, offset))
            {
            }
        }

        public static Node Parse(string text)
        {
            var p = new Parser(text);
            p.SkipWs();
            var node = p.Value();
            p.SkipWs();
            if (!p.AtEnd)
            {
                throw new ParseException("trailing content after document", p.Pos);
            }
            return node;
        }

        private sealed class Parser
        {
            private readonly string _s;
            private int _i;

            public Parser(string s)
            {
                _s = s;
            }

            public int Pos
            {
                get
                {
                    return _i;
                }
            }

            public bool AtEnd
            {
                get
                {
                    return _i >= _s.Length;
                }
            }

            public void SkipWs()
            {
                while (_i < _s.Length && (_s[_i] == ' ' || _s[_i] == '\t' || _s[_i] == '\n' || _s[_i] == '\r'))
                {
                    _i++;
                }
            }

            public Node Value()
            {
                if (_i >= _s.Length)
                {
                    throw new ParseException("unexpected end of input", _i);
                }
                char c = _s[_i];
                switch (c)
                {
                    case '{':
                        return Obj();
                    case '[':
                        return Arr();
                    case '"':
                        return Node.StrNode(Str());
                    case 't':
                        Literal("true");
                        return Node.BoolNode(true);
                    case 'f':
                        Literal("false");
                        return Node.BoolNode(false);
                    case 'n':
                        Literal("null");
                        return Node.NullNode();
                    default:
                        if (c == '-' || (c >= '0' && c <= '9'))
                        {
                            return Num();
                        }
                        throw new ParseException("unexpected character '" + c + "'", _i);
                }
            }

            private Node Obj()
            {
                _i++;
                var map = new Dictionary<string, Node>(StringComparer.Ordinal);
                SkipWs();
                if (_i < _s.Length && _s[_i] == '}')
                {
                    _i++;
                    return Node.ObjNode(map);
                }
                while (true)
                {
                    SkipWs();
                    if (_i >= _s.Length || _s[_i] != '"')
                    {
                        throw new ParseException("object key must be a string", _i);
                    }
                    string key = Str();
                    SkipWs();
                    Expect(':');
                    SkipWs();
                    var v = Value();
                    if (map.ContainsKey(key))
                    {
                        throw new ParseException("duplicate key \"" + key + "\"", _i);
                    }
                    map[key] = v;
                    SkipWs();
                    if (_i < _s.Length && _s[_i] == ',')
                    {
                        _i++;
                        continue;
                    }
                    Expect('}');
                    return Node.ObjNode(map);
                }
            }

            private Node Arr()
            {
                _i++;
                var list = new List<Node>();
                SkipWs();
                if (_i < _s.Length && _s[_i] == ']')
                {
                    _i++;
                    return Node.ArrNode(list);
                }
                while (true)
                {
                    SkipWs();
                    list.Add(Value());
                    SkipWs();
                    if (_i < _s.Length && _s[_i] == ',')
                    {
                        _i++;
                        continue;
                    }
                    Expect(']');
                    return Node.ArrNode(list);
                }
            }

            private string Str()
            {
                Expect('"');
                var sb = new StringBuilder();
                while (_i < _s.Length)
                {
                    char c = _s[_i++];
                    if (c == '"')
                    {
                        return sb.ToString();
                    }
                    if (c == '\\')
                    {
                        if (_i >= _s.Length)
                        {
                            break;
                        }
                        char e = _s[_i++];
                        switch (e)
                        {
                            case '"': sb.Append('"'); break;
                            case '\\': sb.Append('\\'); break;
                            case '/': sb.Append('/'); break;
                            case 'b': sb.Append('\b'); break;
                            case 'f': sb.Append('\f'); break;
                            case 'n': sb.Append('\n'); break;
                            case 'r': sb.Append('\r'); break;
                            case 't': sb.Append('\t'); break;
                            case 'u':
                                if (_i + 4 > _s.Length)
                                {
                                    throw new ParseException("truncated \\u escape", _i);
                                }
                                sb.Append((char)Convert.ToInt32(_s.Substring(_i, 4), 16));
                                _i += 4;
                                break;
                            default:
                                throw new ParseException("bad escape \\" + e, _i - 1);
                        }
                    }
                    else
                    {
                        sb.Append(c);
                    }
                }
                throw new ParseException("unterminated string", _i);
            }

            private Node Num()
            {
                int start = _i;
                if (_s[_i] == '-')
                {
                    _i++;
                }
                while (_i < _s.Length && (((_s[_i] >= '0') && (_s[_i] <= '9'))
                    || _s[_i] == '.' || _s[_i] == 'e' || _s[_i] == 'E' || _s[_i] == '+' || _s[_i] == '-'))
                {
                    _i++;
                }
                string tok = _s.Substring(start, _i - start);
                if (!double.TryParse(tok, NumberStyles.Float, CultureInfo.InvariantCulture, out double v))
                {
                    throw new ParseException("invalid number \"" + tok + "\"", start);
                }
                return Node.NumNode(v);
            }

            private void Literal(string lit)
            {
                if (_i + lit.Length > _s.Length || string.CompareOrdinal(_s, _i, lit, 0, lit.Length) != 0)
                {
                    throw new ParseException("expected " + lit, _i);
                }
                _i += lit.Length;
            }

            private void Expect(char c)
            {
                if (_i >= _s.Length || _s[_i] != c)
                {
                    throw new ParseException("expected '" + c + "'", _i);
                }
                _i++;
            }
        }
    }
}
