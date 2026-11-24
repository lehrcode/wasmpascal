program Prozeduren;
type
  TFeld = array[1..5] of Integer;
var
  g: Integer;

procedure Leer;
begin
end;

procedure Modi(a: Integer; var b: Integer; const c: TFeld; var d; const e);
var
  lokal: Real;
begin
  b := a + c[1];
  Integer(d) := 0;
  lokal := Byte(e)
end;

function Quadrat(x: Integer): LongInt; forward;

function Summe(const f: TFeld): Integer;
var
  x: Integer;
begin
  Result := 0;
  for x in f do Inc(Result, Quadrat(x))
end;

function Quadrat;
begin
  Quadrat := x * x
end;

procedure Aussen(n: Integer);
var
  Tiefe: Integer;

  function Mitte(k: Integer): Integer;

    procedure Innen;
    begin
      Inc(Tiefe, n + k)
    end;

  begin
    Innen;
    Mitte := Tiefe
  end;

begin
  Tiefe := 0;
  g := Mitte(n)
end;

type
  TPunkt = record x, y: Integer end;
  TZiffern = set of 0..9;

function Punkt(x, y: Integer): TPunkt;
begin
  Result.x := x;
  Result.y := y
end;

function Quadrate: TFeld;
var
  i: Integer;
begin
  for i := 1 to 5 do Result[i] := i * i
end;

function Gerade: TZiffern;
begin
  Gerade := [0, 2, 4, 6, 8]
end;

function Zaehlen: Integer;
begin
  Inc(g);
  Zaehlen := g
end;

function Text(s: string): string;
begin
  Text := s + '!'
end;

var
  p: TPunkt;
  q: TFeld;
begin
  Leer;
  Aussen(3);
  p := Punkt(1, 2);
  g := Punkt(3, 4).y + Quadrate[2] + Summe(Quadrate);
  q := Quadrate;
  if 4 in Gerade then Zaehlen;
  Zaehlen;
  Quadrat(7)
end.
