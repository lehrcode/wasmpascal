program ProzedurTypen;
type
  TProc = procedure;
  TFunc = function(X: Real): Real;
  TZahl = function: Integer;
var
  P: TProc;
  F: TFunc;
  Z: TZahl;
  I: Integer;

procedure Hallo;
begin
end;

function Quadrat(X: Real): Real;
begin
  Quadrat := X * X
end;

function Eins: Integer;
begin
  Eins := 1
end;

const
  K: TFunc = Quadrat;

begin
  P := Hallo;
  F := Quadrat;
  Z := Eins;
  I := Z;
  WriteLn(F(2), K(3), @P = nil)
end.
