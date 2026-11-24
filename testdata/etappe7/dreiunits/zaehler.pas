unit Zaehler;

interface

uses Werkzeug;

var
  Stand: Integer;
  Ursprung: TPunkt;

procedure Zaehle(n: Integer);
function AlsText: string;

implementation

uses Protokoll;

var
  Intern: Integer;

procedure Zaehle(n: Integer);
begin
  Stand := Stand + n;
  Inc(Intern);
  Notiere('gezaehlt')
end;

function AlsText: string;
begin
  Str(Stand, Result)
end;

begin
  Stand := 10;
  Intern := 0;
  Ursprung.x := 1;
  Ursprung.y := 2;
  WriteLn('Init Zaehler')
end.
