program Schleifen;
type
  TFeld = array[1..6] of Integer;
const
  Werte: TFeld = (3, -1, 4, -1, 5, 9);
var
  i, j, Summe: Integer;
  w: Integer;
  lokal: array[0..2] of Integer;

function SummePositiv(const f: TFeld): Integer;
var
  x: Integer;
begin
  Result := 0;
  for x in f do
  begin
    if x < 0 then Continue;
    Result := Result + x
  end
end;

function ErsterNegativer(const f: TFeld): Integer;
var
  i: Integer;
begin
  Result := 0;
  for i := Low(f) to High(f) do
    if f[i] < 0 then
    begin
      Result := i;
      Break
    end
end;

function Suche(const f: TFeld; Wert: Integer): Integer;
var
  i: Integer;
begin
  for i := 1 to 6 do
    if f[i] = Wert then
    begin
      Suche := i;
      Exit
    end;
  Suche := -1
end;

begin
  WriteLn('Summe positiv: ', SummePositiv(Werte));
  WriteLn('erster negativer bei ', ErsterNegativer(Werte));
  WriteLn('5 bei ', Suche(Werte, 5), ', 7 bei ', Suche(Werte, 7));
  for i := 10 downto 7 do Write(i, ' ');
  WriteLn;
  for i := 5 to 4 do WriteLn('nie');
  Summe := 0;
  i := 0;
  while True do
  begin
    Inc(i);
    if i > 100 then Break;
    if Odd(i) then Continue;
    Summe := Summe + i
  end;
  WriteLn('gerade Summe bis 100: ', Summe);
  i := 0;
  repeat
    Inc(i);
    if i = 3 then Continue;
    Write(i);
    if i = 5 then Break
  until i >= 10;
  WriteLn;
  for i := 1 to 3 do
  begin
    for j := 1 to 3 do
    begin
      if j > i then Break;
      Write(i * j:3)
    end;
    WriteLn
  end;
  for i := 0 to 2 do lokal[i] := i * i;
  for w in lokal do Write(w, ';');
  WriteLn;
  for w in Werte do
    if w > 4 then Write(w, ' ');
  WriteLn
end.
