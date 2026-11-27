{ Die Exit-Prozedur einer Unit läuft nach der des Programms }
program ExitUnit;

uses Protokoll;

var
  Alt: Pointer;

procedure Laut(const S: string);
begin
  WriteLn('!! ', S)
end;

procedure ProgrammEnde;
begin
  ExitProc := Alt;
  Melde('Programmende')
end;

begin
  Alt := ExitProc;
  ExitProc := @ProgrammEnde;
  Melde('eins');
  Melder := Laut;
  Melde('zwei');
  Halt(2)
end.
