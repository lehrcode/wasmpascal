unit Geometrie;

interface

uses Crt;

const
  Pi2 = 6.283185307179586;

type
  TPunkt = record
    x, y: Real
  end;

var
  Zaehler: Integer;

function Abstand(const a, b: TPunkt): Real;
procedure Verschiebe(var p: TPunkt; dx, dy: Real);

implementation

uses Dos;

const
  Epsilon = 1E-9;

function Quadrat(x: Real): Real;
begin
  Quadrat := x * x
end;

function Abstand(const a, b: TPunkt): Real;
begin
  Inc(Zaehler);
  Abstand := Sqrt(Quadrat(a.x - b.x) + Quadrat(a.y - b.y))
end;

procedure Verschiebe;
begin
  p.x := p.x + dx;
  p.y := p.y + dy
end;

begin
  Zaehler := 0
end.
