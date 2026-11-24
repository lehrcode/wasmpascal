program Mengen;
type
  TFarbe = (Rot, Gelb, Gruen, Blau, Weiss);
  TFarben = set of TFarbe;
  TZiffern = set of 0..9;
var
  a, b, c: TZiffern;
  f: TFarben;
  farbe: TFarbe;
  z: Integer;
  Zeichen: set of Char;
  ch: Char;
  lo, hi: Integer;

procedure Zeige(const s: TZiffern);
var
  i: Integer;
begin
  Write('[');
  for i in s do Write(i);
  WriteLn(']')
end;

function Gerade: TZiffern;
begin
  Gerade := [0, 2, 4, 6, 8]
end;

begin
  a := [1, 2, 3, 4, 5];
  b := Gerade;
  Zeige(a + b);
  Zeige(a * b);
  Zeige(a - b);
  Zeige(b - a);
  Zeige([]);
  lo := 3; hi := 6;
  c := [lo..hi, 9, lo - 3];
  Zeige(c);
  WriteLn(a = b, ' ', a <> b, ' ', [2, 4] <= b, ' ', b >= [2, 4], ' ', a <= b);
  WriteLn(a * b = [2, 4], ' ', [] <= a, ' ', a + [] = a);
  z := 7;
  WriteLn(z in a, ' ', z in c + [7], ' ', 4 in a * b, ' ', 300 in c);
  f := [Rot..Gruen] - [Gelb];
  f := f + [Weiss];
  for farbe in f do Write(Ord(farbe));
  WriteLn;
  Zeichen := ['a'..'e', 'x'];
  for ch in 'paxe' do
    if ch in Zeichen then Write(ch) else Write('-');
  WriteLn;
  a := [];
  for z := 0 to 9 do
    if Odd(z) then a := a + [z];
  Zeige(a);
  for z in a do
    a := a - [z];
  Zeige(a)
end.
