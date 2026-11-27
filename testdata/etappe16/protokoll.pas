{ Unit mit eigener Exit-Prozedur, wie in TP üblich }
unit Protokoll;

interface

type
  TMelder = procedure(const S: string);

var
  Melder: TMelder;

procedure Melde(const S: string);

implementation

var
  AltExit: Pointer;
  Anzahl: Integer;

procedure Standard(const S: string);
begin
  WriteLn('[log] ', S)
end;

procedure Melde(const S: string);
begin
  Inc(Anzahl);
  Melder(S)
end;

procedure ProtokollEnde;
begin
  ExitProc := AltExit;
  WriteLn('[log] ', Anzahl, ' Meldungen, ExitCode ', ExitCode)
end;

begin
  Melder := Standard;
  Anzahl := 0;
  AltExit := ExitProc;
  ExitProc := @ProtokollEnde
end.
