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

function Text(s: string): string;
begin
  Text := s + '!'
end;

begin
  Leer;
  Aussen(3)
end.
