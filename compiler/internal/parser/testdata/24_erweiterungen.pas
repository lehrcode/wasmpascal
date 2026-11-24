program Erweiterungen;
type
  TFeld = array[1..5] of Integer;
  TPunkt = record x, y: Integer end;
const
  Werte: TFeld = (3, -1, 4, -1, 5);
var
  Ziffern: set of 0..9;
  z: Integer;
  ch: Char;

function Summe(const f: TFeld): Integer;
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
  ErsterNegativer := 0;
  for i := Low(f) to High(f) do
    if f[i] < 0 then
    begin
      Result := i;
      Break
    end
end;

procedure Zeige(const p: TPunkt; const Titel: string);
begin
  WriteLn(Titel, ': ', p.x, ',', p.y)
end;

begin
  WriteLn(Summe(Werte), ' ', ErsterNegativer(Werte));
  for ch in 'abc' do Write(UpCase(ch));
  WriteLn;
  Ziffern := [1, 3, 5];
  for z in Ziffern do Write(z);
  WriteLn;
  while True do
  begin
    z := z + 1;
    if z > 10 then Break
  end;
  repeat
    Dec(z);
    if Odd(z) then Continue
  until z = 0
end.
