{ Indirekte Aufrufe von Routinen, die anhalten (ReadLn, lange Ausgaben):
  Beim Fortsetzen trifft der Aufruf dieselbe Routine, auch wenn sich die
  Prozedurvariable inzwischen geändert hat. }
program Anhalten;

type
  TLeser = procedure(var S: string);
  TFunc = function(N: Integer): Integer;

var
  L: TLeser;
  F: TFunc;
  S: string;
  I: Integer;

procedure Lies(var S: string);
begin
  L := nil;             { Variable ändern, bevor angehalten wird }
  ReadLn(S)
end;

procedure Anders(var S: string);
begin
  S := 'falsch'
end;

function Zeilen(N: Integer): Integer;
var
  K: Integer;
begin
  for K := 1 to N do
    WriteLn('Zeile ', K);
  Zeilen := N * 2
end;

function Doppelt(N: Integer): Integer;
begin
  Doppelt := N + N
end;

procedure Ende;
var
  T: string;
begin
  Write('Name? ');
  ReadLn(T);
  WriteLn('Tschuess, ', T)
end;

begin
  L := Lies;
  L(S);
  WriteLn('gelesen: ', S, ' ', @L = nil);
  L := Anders;
  L(S);
  WriteLn(S);
  F := Zeilen;
  I := F(3);
  F := Doppelt;
  WriteLn(I, ' ', F(I));
  ExitProc := @Ende
end.
