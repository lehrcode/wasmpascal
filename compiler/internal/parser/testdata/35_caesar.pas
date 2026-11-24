program Caesar(Input, Output);
const
  Verschiebung = 3;
var
  Klar, Geheim: string;

function Verschluessle(const s: string; k: Integer): string;
var
  i: Integer;
  c: Char;
  t: string;
begin
  t := s;
  for i := 1 to Length(s) do
  begin
    c := UpCase(s[i]);
    if c in ['A'..'Z'] then
      t[i] := Chr((Ord(c) - Ord('A') + k + 26) mod 26 + Ord('A'))
  end;
  Verschluessle := t
end;

begin
  ReadLn(Klar);
  Geheim := Verschluessle(Klar, Verschiebung);
  WriteLn(Geheim);
  WriteLn(Verschluessle(Geheim, -Verschiebung))
end.
