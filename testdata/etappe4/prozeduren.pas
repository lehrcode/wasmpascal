program Prozeduren;
type
  TFeld = array[1..5] of Integer;
  TMatrix = array[1..2, 1..3] of Integer;
var
  a, b: Integer;
  f, g: TFeld;
  m: TMatrix;
  i, j: Integer;
  Zaehler: Integer;

procedure Tausche(var x, y: Integer);
var
  h: Integer;
begin
  h := x; x := y; y := h
end;

procedure Verdopple(var f: TFeld);
var
  i: Integer;
begin
  for i := 1 to 5 do f[i] := 2 * f[i]
end;

procedure Kopie(f: TFeld);
begin
  f[1] := 999;
  Write(f[1], ' ')
end;

function Quadrate: TFeld;
var
  i: Integer;
begin
  for i := 1 to 5 do Result[i] := i * i
end;

function Summe(const f: TFeld): Integer;
var
  i, s: Integer;
begin
  s := 0;
  for i := 1 to 5 do s := s + f[i];
  Summe := s
end;

function Zaehle: Integer;
begin
  Inc(Zaehler);
  Zaehle := Zaehler
end;

procedure Aussen;

  procedure Innen(n: Integer);
  begin
    Write('innen ', n, ' ')
  end;

begin
  Innen(1);
  Innen(2);
  WriteLn
end;

function Ggt(a, b: Integer): Integer; forward;

procedure Zeige(x, y: Integer);
begin
  WriteLn('ggT(', x, ', ', y, ') = ', Ggt(x, y))
end;

function Ggt(a, b: Integer): Integer;
begin
  if b = 0 then Ggt := a else Ggt := Ggt(b, a mod b)
end;

procedure Inkrement(var x: Integer; n: Integer);
begin
  x := x + n
end;

begin
  a := 1; b := 2;
  Tausche(a, b);
  WriteLn(a, ' ', b);
  f := Quadrate;
  WriteLn(Summe(f), ' ', Summe(Quadrate), ' ', Quadrate[3]);
  Verdopple(f);
  Kopie(f);
  WriteLn(f[1], ' ', f[5]);
  g := f;
  g[2] := 0;
  WriteLn(f[2], ' ', g[2]);
  Tausche(f[1], f[5]);
  WriteLn(f[1], ' ', f[5]);
  for i := 1 to 2 do
    for j := 1 to 3 do
      m[i, j] := 10 * i + j;
  WriteLn(m[1, 3], ' ', m[2][1]);
  Inkrement(m[2, 2], 100);
  WriteLn(m[2, 2]);
  Zaehler := 0;
  Zaehle;
  Zaehle;
  WriteLn('Zaehler: ', Zaehle);
  Aussen;
  Zeige(84, 36)
end.
