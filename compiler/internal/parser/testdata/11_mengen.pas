program Mengen;
type
  TZiffern = set of 0..9;
  TBuchstaben = set of Char;
var
  Gerade, Prim, Beide: TZiffern;
  Vokale, Gesehen: TBuchstaben;
  c: Char;
  i: Integer;
  s: string;
begin
  Gerade := [0, 2, 4, 6, 8];
  Prim := [2, 3, 5, 7];
  Beide := Gerade * Prim;
  Gerade := Gerade + [10 - 1] - [0];
  if Beide = [2] then WriteLn('nur die 2');
  if [2] <= Prim then WriteLn('Teilmenge');
  for i := 0 to 9 do
    if i in Gerade + Prim then Write(i);
  WriteLn;
  Vokale := ['a', 'e', 'i', 'o', 'u', 'A'..'Z'];
  Gesehen := [];
  s := 'Mengenlehre';
  for i := 1 to Length(s) do
  begin
    c := s[i];
    if (c in Vokale) and not (c in Gesehen) then
    begin
      Gesehen := Gesehen + [c];
      Write(c)
    end
  end;
  WriteLn
end.
