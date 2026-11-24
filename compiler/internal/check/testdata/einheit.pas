unit Einheit;

interface

const
  Version = 2;

type
  TPunkt = record
    x, y: Real
  end;

var
  Anzahl: Integer;

function Abstand(const a, b: TPunkt): Real;
procedure Zaehle;

implementation

var
  Intern: Integer;

function Quadrat(x: Real): Real;
begin
  Quadrat := x * x
end;

function Abstand;
begin
  Abstand := Sqrt(Quadrat(a.x - b.x) + Quadrat(a.y - b.y))
end;

procedure Zaehle;
begin
  Inc(Anzahl);
  Inc(Intern)
end;

begin
  Anzahl := 0;
  Intern := Version
end.
